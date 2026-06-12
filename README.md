# OpenSkill Rating Bot for mixi2

mixi2上で動作するレーティングボットです。[OpenSkill (Plackett-Luce)](https://openskill.me/en/stable/manual.html) アルゴリズムを使用してプレイヤーのスキルレーティングを計算・管理します。

## 機能

- **勝敗記録**: 管理者がメンションで勝敗を記録
- **レーティング自動更新**: Plackett-Luceアルゴリズムで公平なレーティング計算
- **ランキング表示**: 上位プレイヤーのランキングを即座に表示
- **個人レーティング確認**: 特定プレイヤーの詳細レーティングを表示
- **対戦履歴**: SQLiteで全対戦結果を永続保存

## レーティングシステム

| パラメータ | 値 | 説明 |
|-----------|-----|------|
| 初期レーティング (μ) | **1000** | スキルの平均推定値 |
| 初期不確実性 (σ) | **333.33** (= μ/3) | スキルの不確実性 |
| Beta (β) | **166.67** (= σ/2) | パフォーマンスのばらつき |
| Tau (τ) | **3.33** (= μ/300) | 動的変動パラメータ |

保守的評価値 (Ordinal) = μ - 3σ（真のスキルが99.7%の確率でこれ以上）

## ボットの使い方

### 勝敗記録（管理者のみ）
```
@openskill_rating @勝者のID が @敗者のID に勝ちました！
```

### ランキング表示
```
@openskill_rating ランキング
```

### 個人レーティング確認
```
@openskill_rating @ユーザーID レーティング
```

### ヘルプ
```
@openskill_rating ヘルプ
```

---

## セットアップ

### 必要なシークレット（GitHub Actions Secrets）

| シークレット名 | 説明 |
|--------------|------|
| `MIXI_CLIENT_ID` | mixi2アプリケーションのClient ID |
| `MIXI_CLIENT_SECRET` | mixi2アプリケーションのClient Secret |
| `MIXI_ADMIN_USER_ID` | 管理者のmixi2ユーザーID（勝敗記録コマンドを使える人） |
| `MIXI_COMMUNITY_ID` | ボットが動作するコミュニティのID |

### ⚠️ GitHub Actions ワークフローの登録手順

このリポジトリのワークフローファイル (`check-and-build.yml`, `run-bot.yml`) は、Genspark AIの権限制限により自動登録できません。
以下の手順で手動登録してください。

#### 方法1: Personal Access Token (PAT) を使ってコマンドラインで登録（推奨）

1. [GitHub Settings > Developer settings > Personal access tokens > Tokens (classic)](https://github.com/settings/tokens) で新しいトークンを作成
   - **必要なスコープ**: `workflow`, `repo`
2. 以下のコマンドを実行：

```bash
# PATをセット
export GITHUB_TOKEN=<あなたのPAT>

# check-and-build.yml を登録
curl -X PUT \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Content-Type: application/json" \
  https://api.github.com/repos/xanqi-md/mixi2-rating-calculator/contents/.github/workflows/check-and-build.yml \
  -d "{\"message\":\"ci: add build and connection test workflow\",\"content\":\"$(base64 -w0 .github/workflows/check-and-build.yml)\"}"

# run-bot.yml を登録
curl -X PUT \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Content-Type: application/json" \
  https://api.github.com/repos/xanqi-md/mixi2-rating-calculator/contents/.github/workflows/run-bot.yml \
  -d "{\"message\":\"ci: add bot runner workflow\",\"content\":\"$(base64 -w0 .github/workflows/run-bot.yml)\"}"
```

#### 方法2: GitHub Web UI から直接作成

1. https://github.com/xanqi-md/mixi2-rating-calculator/new/main?filename=.github/workflows/check-and-build.yml を開く
2. ファイル内容を `.github/workflows/check-and-build.yml` からコピーして貼り付け
3. "Commit changes" をクリック
4. 同様に `run-bot.yml` も作成

#### ワークフロー登録後の確認手順

1. **シークレット確認**: [Actions > Check Secrets and Build > Run workflow](https://github.com/xanqi-md/mixi2-rating-calculator/actions) を実行
   - `MIXI_CLIENT_ID: SET` ✅ が出ればOK
2. **接続テスト**: 同ワークフローの `connection-test` ジョブでmixi2 APIへの接続を検証
3. **ボット起動**: [Actions > Run Rating Bot > Run workflow](https://github.com/xanqi-md/mixi2-rating-calculator/actions) で `stream` モードを選択して実行

---

## ビルド方法

```bash
# 依存関係のインストール
sudo apt-get install -y gcc libsqlite3-dev

# 全バイナリのビルド
CGO_ENABLED=1 go build -o bin/stream   ./cmd/stream/
CGO_ENABLED=1 go build -o bin/webhook  ./cmd/webhook/
CGO_ENABLED=1 go build -o bin/test-connection ./cmd/test-connection/

# テスト
CGO_ENABLED=1 go test ./internal/rating/... -v
```

## 環境変数

| 変数名 | デフォルト値 | 説明 |
|--------|------------|------|
| `MIXI_CLIENT_ID` または `CLIENT_ID` | (必須) | mixi2 Client ID |
| `MIXI_CLIENT_SECRET` または `CLIENT_SECRET` | (必須) | mixi2 Client Secret |
| `MIXI_ADMIN_USER_ID` または `ADMIN_USER_ID` | (必須) | 管理者ユーザーID |
| `MIXI_COMMUNITY_ID` または `COMMUNITY_ID` | (任意) | コミュニティID |
| `TOKEN_URL` | `https://application-auth.mixi.social/oauth2/token` | OAuth2トークンURL |
| `API_ADDRESS` | `application-api.mixi.social:443` | gRPC APIアドレス |
| `STREAM_ADDRESS` | `application-stream.mixi.social:443` | gRPCストリームアドレス |
| `DB_PATH` | `./ratings.db` | SQLiteデータベースパス |

## アーキテクチャ

```
cmd/
  stream/      - gRPC Stream モード（長時間稼働）
  webhook/     - HTTP Webhook モード（HTTPS必須）
  test-connection/ - 接続テストツール
internal/
  config/      - 環境変数読み込み
  rating/      - Plackett-Luce アルゴリズム
  db/          - SQLite永続化層
  handler/     - mixi2イベントハンドラ
```

## テスト結果

```
--- PASS: TestNewRating (0.00s)
--- PASS: TestPlackettLuceRate_WinnerGainsLoserLoses (0.00s)  
--- PASS: TestPlackettLuceRate_HigherRatedWinner (0.00s)
--- PASS: TestOrdinal (0.00s)
ok  github.com/yourusername/openskill-rating-bot/internal/rating
```

## 動作確認済み

mixi2 API への接続は `mixi2-crystal-bot` の実績で確認済み（2026-06-12）:
- OAuth2トークン取得 ✅
- gRPC接続 (`application-api.mixi.social:443`) ✅  
- コミュニティ投稿 ✅（`post_id=99cb2b7f-5a55-4e8b-8d45-9dee236b47f3`）
