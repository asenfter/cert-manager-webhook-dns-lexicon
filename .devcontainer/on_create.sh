#! /bin/bash

source .devcontainer/.env

git config --global user.name "$MY_GIT_NAME"
git config --global user.email "$MY_GIT_EMAIL"
git config --global core.ignorecase false

# Install packages required for installations
apt-get update && apt-get install -y curl make gettext-base ca-certificates curl
# ----------------------------------------------------------------
# Install Docker CLI
# Add Docker's official GPG key:
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
# Add the repository to Apt sources:
tee /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/debian
Suites: $(. /etc/os-release && echo "$VERSION_CODENAME")
Components: stable
Signed-By: /etc/apt/keyrings/docker.asc
EOF

apt update
apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
# ----------------------------------------------------------------
# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/arm64/kubectl"
echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check
install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
kubectl version --client
kubectl version --client --output=yaml
# Clean up
rm -f kubectl kubectl.sha256
# ----------------------------------------------------------------
# Install helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
helm version
# ----------------------------------------------------------------