package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	cmd.RunWebhookServer(GroupName, &customDNSProviderSolver{})
}

type customDNSProviderSolver struct {
	client kubernetes.Interface
}

type customDNSProviderConfig struct {
	// dns-lexicon provider, e.g. "hetzner" or "desec"
	Provider string `json:"provider"`

	// optional override, else use ch.ResolvedZone
	ZoneName string `json:"zoneName,omitempty"`

	// token via secretRef
	AuthTokenSecretRef cmmeta.SecretKeySelector `json:"authTokenSecretRef"`

	// optional
	TTL int `json:"ttl,omitempty"`
}

func (c *customDNSProviderSolver) Name() string {
	return "dns-lexicon"
}

func (c *customDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	c.client = cl
	return nil
}

func (c *customDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}
	if cfg.Provider == "" {
		return fmt.Errorf("config.provider must be set (e.g. hetzner, desec)")
	}

	zone := strings.TrimSuffix(cfg.ZoneName, ".")
	if zone == "" {
		zone = strings.TrimSuffix(ch.ResolvedZone, ".")
	}
	if zone == "" {
		return fmt.Errorf("could not determine zone (config.zoneName or ch.ResolvedZone)")
	}

	// FIX: ensure we always set the TXT record under _acme-challenge
	name, err := recordNameForChallenge(ch.ResolvedFQDN, zone)
	if err != nil {
		return err
	}

	token, err := c.readSecretKey(ch.ResourceNamespace, cfg.AuthTokenSecretRef)
	if err != nil {
		return err
	}

	ttl := cfg.TTL
	if ttl == 0 {
		ttl = 60
	}

	// Create; if it fails (record exists), try update.
	if err := c.lexicon(token,
		cfg.Provider,"create", zone, "TXT",
		"--name", name,
		"--content", ch.Key,
		"--ttl", fmt.Sprintf("%d", ttl),
	); err != nil {
		if err2 := c.lexicon(token,
			cfg.Provider, "update", zone, "TXT",
			"--name", name,
			"--content", ch.Key,
			"--ttl", fmt.Sprintf("%d", ttl),
		); err2 != nil {
			return fmt.Errorf("lexicon create failed: %v; lexicon update failed: %v", err, err2)
		}
	}
	return nil
}

func (c *customDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}
	if cfg.Provider == "" {
		return fmt.Errorf("config.provider must be set (e.g. hetzner, desec)")
	}

	zone := strings.TrimSuffix(cfg.ZoneName, ".")
	if zone == "" {
		zone = strings.TrimSuffix(ch.ResolvedZone, ".")
	}
	if zone == "" {
		return fmt.Errorf("could not determine zone (config.zoneName or ch.ResolvedZone)")
	}

	// same logic as Present
	name, err := recordNameForChallenge(ch.ResolvedFQDN, zone)
	if err != nil {
		return err
	}

	token, err := c.readSecretKey(ch.ResourceNamespace, cfg.AuthTokenSecretRef)
	if err != nil {
		return err
	}

	// Best-effort delete; ignore errors (some providers return not found etc.)
	_ = c.lexicon(cfg.Provider, token,
		"delete", zone, "TXT",
		"--name", name,
		"--content", ch.Key,
	)
	return nil
}

func (c *customDNSProviderSolver) readSecretKey(ns string, ref cmmeta.SecretKeySelector) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("kubernetes client not initialized")
	}
	// Don't silently fall back to default; it hides bugs and causes wrong secrets to be read.
	if ns == "" {
		return "", fmt.Errorf("resource namespace is empty; cannot read secret")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secName := ref.LocalObjectReference.Name
	if secName == "" {
		return "", fmt.Errorf("authTokenSecretRef.name must be set")
	}
	if ref.Key == "" {
		return "", fmt.Errorf("authTokenSecretRef.key must be set")
	}

	sec, err := c.client.CoreV1().Secrets(ns).Get(ctx, secName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", ns, secName, err)
	}
	val, ok := sec.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s missing key %q", ns, secName, ref.Key)
	}
	return strings.TrimSpace(string(val)), nil
}

func sanitizeLexiconArgs(args []string) []string {
	out := make([]string, 0, len(args))
	skipNext := false

	for i := 0; i < len(args); i++ {
		if skipNext {
			skipNext = false
			continue
		}
		switch args[i] {
		case "--auth-token", "--auth-username", "--auth-password":
			out = append(out, args[i], "<redacted>")
			skipNext = true
		default:
			out = append(out, args[i])
		}
	}
	return out
}

func (c *customDNSProviderSolver) lexicon(token string, args ...string) error {
	// lexicon_cmd: <provider> <action> <zone> TXT --name ... --content ... --ttl ...

	cmd := exec.Command("lexicon", args...)
	cmd.Env = append(os.Environ(), "LEXICON_LOG_LEVEL=warning")

	// token-only (hetzner + desec typically)
	cmd.Args = append(cmd.Args, "--auth-token", token)

	fmt.Printf(
		"----> [Call LEXICON] provider=%s action=%s zone=%s args=%v\n",
		cmd.Args[1], // provider
		cmd.Args[2], // create / update / delete
		cmd.Args[3], // zone
		sanitizeLexiconArgs(cmd.Args),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lexicon failed: %w; output=%s", err, string(out))
	}
	return nil
}

// recordNameForChallenge returns the relative record name inside the zone.
// It also enforces the _acme-challenge prefix if the provided FQDN does not include it.
func recordNameForChallenge(resolvedFQDN, zone string) (string, error) {
	rel := relativeRecordName(resolvedFQDN, zone)
	rel = strings.TrimSuffix(rel, ".")
	if rel == "" {
		// edge case: root record -> _acme-challenge at zone apex
		return "_acme-challenge", nil
	}

	// If the fixture/webhook request already includes _acme-challenge, keep it.
	if rel == "_acme-challenge" || strings.HasPrefix(rel, "_acme-challenge.") {
		return rel, nil
	}

	// Otherwise, enforce ACME TXT prefix.
	return "_acme-challenge." + rel, nil
}

func relativeRecordName(resolvedFQDN, zone string) string {
	fqdn := strings.TrimSuffix(resolvedFQDN, ".")
	z := strings.TrimSuffix(zone, ".")
	suffix := "." + z
	if strings.HasSuffix(fqdn, suffix) {
		name := strings.TrimSuffix(fqdn, suffix)
		return strings.TrimSuffix(name, ".")
	}
	return fqdn
}

func loadConfig(cfgJSON *extapi.JSON) (customDNSProviderConfig, error) {
	cfg := customDNSProviderConfig{}
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}
	return cfg, nil
}
