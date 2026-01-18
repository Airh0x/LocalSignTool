# 環境構築簡素化の提案

現状のセットアップ課題と改善案をまとめました。

## 現状の課題

1. **依存関係の手動インストール**
   - Go 1.24.0以上
   - fastlane (Ruby/Gem)
   - Python 3
   - Node.js + npm依存関係

2. **プロファイルの手動セットアップ**
   - 証明書ファイル（cert.p12）
   - パスワードファイル
   - アカウント情報
   - ファイル権限の設定

3. **ビルドプロセスの理解**
   - `go mod download` → `go build` → `build_app.sh`
   - どのステップが必要かの判断

## 提案：環境構築の簡素化案

### 案1: 自動セットアップスクリプト（推奨）⭐

**概要**: 依存関係のチェック・インストールを自動化

**実装内容**:
- `setup.sh` スクリプトを作成
  - 必要なツールの存在確認（Go, fastlane, Python3, Node.js）
  - 不足しているものの自動インストール案内
  - npm依存関係の自動インストール（`builder/node-utils/`）
  - ビルドの自動実行
  - 初回設定ウィザード（プロファイル作成支援）

**メリット**:
- ワンコマンドで環境構築完了
- 新規ユーザーが迷わない
- エラーメッセージが分かりやすい

**使用例**:
```bash
./setup.sh
# または対話形式でプロファイルも設定
./setup.sh --interactive
```

---

### 案2: 依存関係チェック機能（実行時チェック）

**概要**: アプリケーション起動時に依存関係をチェック

**実装内容**:
- `main.go`の起動時に依存関係をチェック
  - Go実行ファイル、fastlane、Python3、Node.js、npmパッケージ
- 不足があれば分かりやすいエラーメッセージと解決方法を表示

**メリット**:
- 実行時エラーを事前に防げる
- ユーザーが何が足りないかすぐ分かる

**使用例**:
```bash
./SignTools
# 出力例:
# Error: fastlane not found. Install with: brew install fastlane
# Error: npm dependencies not installed. Run: npm install --prefix builder/node-utils
```

---

### 案3: 事前ビルド済みバイナリの提供（GitHub Releases）

**概要**: Goのインストール不要にする

**実装内容**:
- GitHub Actionsで自動ビルド
- GitHub Releasesにバイナリをアップロード
- ダウンロードして実行するだけの構成

**メリット**:
- Goが不要（ビルド済み）
- ダウンロード即実行が可能

**デメリット**:
- リリースプロセスの追加
- macOS用バイナリのみ（クロスコンパイルは現実的でない）

**使用例**:
```bash
# ダウンロードして解凍
wget https://github.com/.../SignTools-macos.zip
unzip SignTools-macos.zip
chmod +x SignTools
./SignTools
```

---

### 案4: npm依存関係の自動インストール（初回実行時）

**概要**: `sign.py`実行時にnpm依存関係を自動チェック・インストール

**実装内容**:
- `builder/node-utils/`で`node_modules`の存在確認
- 存在しなければ自動で`npm install`を実行
- エラー時は明確なメッセージを表示

**メリット**:
- npm依存関係の手動インストールが不要
- 初回実行時に自動で対応

**注意点**:
- `sign.py`で既に`npm install`を実行している可能性がある（要確認）

---

### 案5: プロファイルセットアップウィザード

**概要**: 対話形式でプロファイルを作成

**実装内容**:
- `setup_profile.sh` または `setup_profile.py`
- 証明書ファイル、パスワード、アカウント情報を対話形式で入力
- ファイル作成と権限設定を自動化

**メリット**:
- プロファイル作成が簡単
- ファイル権限の設定ミスを防止

**使用例**:
```bash
./setup_profile.sh developer_account
# 対話形式で証明書、パスワード、アカウント情報を入力
```

---

### 案6: Homebrew Formula（macOS環境のみ）

**概要**: Homebrewでインストール可能にする

**実装内容**:
- `localsigntools.rb`（Homebrew Formula）を作成
- 依存関係（fastlane等）を自動インストール
- ビルド済みバイナリを提供

**メリット**:
- macOSユーザーにとって簡単
- 依存関係の管理が自動

**デメリット**:
- Homebrewリポジトリへの追加が必要
- macOS限定

**使用例**:
```bash
brew tap yourname/localsigntools
brew install localsigntools
```

---

## 推奨実装順序

1. **案4: npm依存関係の自動インストール**（簡単・影響大）
   - 既存コードへの影響が小さい
   - ユーザーの負担を即座に減らせる

2. **案1: 自動セットアップスクリプト**（中規模・効果大）
   - 新規ユーザーのハードルを大きく下げる
   - READMEを簡素化できる

3. **案2: 依存関係チェック機能**（中規模・効果中）
   - 実行時エラーを減らせる
   - 既存コードへの影響が小さい

4. **案5: プロファイルセットアップウィザード**（オプション）
   - ユーザーの利便性向上
   - 必須ではないが、あると親切

5. **案3/案6: バイナリ配布**（長期的・オプション）
   - Go不要になるが、リリースプロセスの追加が必要

---

## 具体的な実装案（案1+案4の組み合わせ）

### `setup.sh` スクリプト

```bash
#!/bin/bash
# LocalSignTools 自動セットアップスクリプト

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 色の定義
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== LocalSignTools セットアップ ==="
echo ""

# 1. Goのチェック
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go が見つかりません${NC}"
    echo "  Homebrewでインストール: brew install go"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✓ Go ${GO_VERSION} が見つかりました${NC}"

# 2. fastlaneのチェック
if ! command -v fastlane &> /dev/null; then
    echo -e "${YELLOW}⚠ fastlane が見つかりません${NC}"
    echo "  Homebrewでインストール: brew install fastlane"
    echo "  または: gem install fastlane"
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
    echo "  Homebrewでインストール: brew install node"
    exit 1
fi

NODE_VERSION=$(node --version)
echo -e "${GREEN}✓ Node.js ${NODE_VERSION} が見つかりました${NC}"

# 5. npm依存関係のインストール
echo ""
echo "npm依存関係をインストール中..."
if [ ! -d "builder/node-utils/node_modules" ]; then
    cd builder/node-utils
    npm install
    cd "$SCRIPT_DIR"
    echo -e "${GREEN}✓ npm依存関係のインストール完了${NC}"
else
    echo -e "${GREEN}✓ npm依存関係は既にインストール済みです${NC}"
fi

# 6. Go依存関係のダウンロード
echo ""
echo "Go依存関係をダウンロード中..."
go mod download
echo -e "${GREEN}✓ Go依存関係のダウンロード完了${NC}"

# 7. ビルド
echo ""
echo "アプリケーションをビルド中..."
go build -o SignTools
echo -e "${GREEN}✓ ビルド完了${NC}"

# 8. 初回設定の案内
if [ ! -d "data/profiles" ] || [ -z "$(ls -A data/profiles 2>/dev/null)" ]; then
    echo ""
    echo -e "${YELLOW}⚠ プロファイルが設定されていません${NC}"
    echo "  プロファイルの設定方法については README.md を参照してください。"
    echo "  または、./setup_profile.sh を実行して対話形式で設定できます。"
fi

echo ""
echo -e "${GREEN}=== セットアップ完了 ===${NC}"
echo ""
echo "以下の方法で実行できます："
echo "  1. ./SignTools  (サーバーモード)"
echo "  2. ./SignTools -headless -ipa <path> -profile <name> -output <path>  (CLIモード)"
echo ""
```

### `sign.py` の修正（案4の実装）

`sign.py`の`npm install`部分を強化：

```python
def ensure_npm_dependencies():
    """npm依存関係がインストールされていることを確認"""
    node_utils_dir = Path("node-utils")
    node_modules_dir = node_utils_dir / "node_modules"
    
    if not node_modules_dir.exists():
        print("Installing npm dependencies...")
        run_process("npm", "install", cwd=str(node_utils_dir))
        print("✓ npm dependencies installed")
    else:
        # 既にインストール済み
        pass
```

---

## まとめ

最も効果的なのは**案1（自動セットアップスクリプト）と案4（npm依存関係の自動インストール）の組み合わせ**です。

これにより：
- ✅ 新規ユーザーが`./setup.sh`で環境構築完了
- ✅ npm依存関係の手動インストールが不要
- ✅ 不足している依存関係が明確
- ✅ READMEを簡素化できる

実装の優先度：
1. **最優先**: npm依存関係の自動チェック・インストール（`sign.py`）
2. **優先**: `setup.sh`スクリプトの作成
3. **推奨**: `main.go`での起動時依存関係チェック
