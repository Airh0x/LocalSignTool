#!/bin/bash

# LocalSignTools 自動セットアップスクリプト
# 依存関係のチェック、インストール、ビルドを自動化します

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 色の定義
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== LocalSignTools セットアップ ===${NC}"
echo ""

# 1. Goのチェック
echo -e "${BLUE}依存関係をチェック中...${NC}"
echo ""

if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go が見つかりません${NC}"
    echo "  Homebrewでインストール: ${GREEN}brew install go${NC}"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✓ Go ${GO_VERSION} が見つかりました${NC}"

# 2. fastlaneのチェック
if ! command -v fastlane &> /dev/null; then
    echo -e "${YELLOW}⚠ fastlane が見つかりません${NC}"
    echo "  Homebrewでインストール: ${GREEN}brew install fastlane${NC}"
    echo "  または: ${GREEN}gem install fastlane${NC}"
    read -p "続行しますか？ (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    FASTLANE_VERSION=$(fastlane --version | head -n 1)
    echo -e "${GREEN}✓ ${FASTLANE_VERSION} が見つかりました${NC}"
fi

# 3. Python3のチェック
if ! command -v python3 &> /dev/null; then
    echo -e "${RED}✗ Python 3 が見つかりません${NC}"
    echo "  macOSには通常含まれています。手動でインストールしてください。"
    exit 1
fi

PYTHON_VERSION=$(python3 --version)
echo -e "${GREEN}✓ ${PYTHON_VERSION} が見つかりました${NC}"

# 4. Node.jsのチェック
if ! command -v node &> /dev/null; then
    echo -e "${RED}✗ Node.js が見つかりません${NC}"
    echo "  Homebrewでインストール: ${GREEN}brew install node${NC}"
    exit 1
fi

NODE_VERSION=$(node --version)
echo -e "${GREEN}✓ Node.js ${NODE_VERSION} が見つかりました${NC}"

# 5. npmのチェック
if ! command -v npm &> /dev/null; then
    echo -e "${YELLOW}⚠ npm が見つかりません${NC}"
    echo "  npmは通常Node.jsに含まれています。Node.jsのインストールを確認してください。"
    read -p "続行しますか？ (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    NPM_VERSION=$(npm --version)
    echo -e "${GREEN}✓ npm ${NPM_VERSION} が見つかりました${NC}"
fi

# 6. npm依存関係のインストール
echo ""
echo -e "${BLUE}npm依存関係をインストール中...${NC}"
if [ ! -d "builder/node-utils/node_modules" ]; then
    cd builder/node-utils
    npm install
    cd "$SCRIPT_DIR"
    echo -e "${GREEN}✓ npm依存関係のインストール完了${NC}"
else
    echo -e "${GREEN}✓ npm依存関係は既にインストール済みです${NC}"
fi

# 7. Go依存関係のダウンロード
echo ""
echo -e "${BLUE}Go依存関係をダウンロード中...${NC}"
go mod download
echo -e "${GREEN}✓ Go依存関係のダウンロード完了${NC}"

# 8. ビルド
echo ""
echo -e "${BLUE}アプリケーションをビルド中...${NC}"
go build -o SignTools
echo -e "${GREEN}✓ ビルド完了${NC}"

# 9. 初回設定の案内
echo ""
if [ ! -d "data/profiles" ] || [ -z "$(ls -A data/profiles 2>/dev/null)" ]; then
    echo -e "${YELLOW}⚠ プロファイルが設定されていません${NC}"
    echo "  プロファイルの設定方法については ${GREEN}README.md${NC} を参照してください。"
fi

echo ""
echo -e "${GREEN}=== セットアップ完了 ===${NC}"
echo ""
echo "以下の方法で実行できます："
echo "  1. ${GREEN}./SignTools${NC}  (サーバーモード)"
echo "  2. ${GREEN}./SignTools -headless -ipa <path> -profile <name> -output <path>${NC}  (CLIモード)"
echo "  3. ${GREEN}./build_app.sh${NC}  (.appバンドルをビルド)"
echo ""
