#!/bin/bash

echo 'eval "$(task --completion zsh)"' >> ~/.zshrc
echo 'eval "$(fga completion zsh)"' >> ~/.zshrc

# Substitute BIN for your bin directory.
# Substitute VERSION for the current released version.
BIN="/usr/local/bin" && \
VERSION="1.50.1" && \
sudo curl -sSL \
"https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf-$(uname -s)-$(uname -m)" \
-o "${BIN}/buf" && \
sudo chmod +x "${BIN}/buf"
