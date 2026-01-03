package main

import (
	"os"
	"testing"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
)

var zone = os.Getenv("TEST_ZONE_NAME") // z.B. "example.com."

func td(path string) string {
	root := os.Getenv("LEXICON_TESTDATA_ROOT")
	if root == "" {
		return path
	}
	return root + "/" + path
}

func TestRunsSuiteHetzner(t *testing.T) {
	if zone == "" {
		t.Skip("TEST_ZONE_NAME not set")
	}

	fixture := acmetest.NewFixture(&customDNSProviderSolver{},
		acmetest.SetResolvedZone(zone),
		acmetest.SetAllowAmbientCredentials(false),
		acmetest.SetManifestPath(td("testdata/lexicon-hetzner")),
	)

	fixture.RunBasic(t)
	fixture.RunExtended(t)
}

func TestRunsSuiteDeSEC(t *testing.T) {
	if zone == "" {
		t.Skip("TEST_ZONE_NAME not set")
	}

	fixture := acmetest.NewFixture(&customDNSProviderSolver{},
		acmetest.SetResolvedZone(zone),
		acmetest.SetAllowAmbientCredentials(false),
		acmetest.SetManifestPath(td("testdata/lexicon-desec")),
	)

	fixture.RunBasic(t)
	fixture.RunExtended(t)
}
