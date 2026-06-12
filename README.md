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

例:
```
@openskill_rating @player_alice が @player_bob に勝ちました！
```

### ランキング確認
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

## セットアップ

### 1. mixi2 Developer登録

1. [mixi2 Developer Platform](https://developer.mixi.social/) にアクセス
2. 開発者登録を行い、新しいアプリケーション（Bot）を作成
3. プラグインIDを `openskill_rating` として設定
4. Client ID・Client Secretを取得

### 2. 環境変数の設定

```bash
cp .env.example .env
```

`.env` を編集して必要な値を設定:

| 変数名 | 必須 | 説明 |
|--------|------|------|
| `CLIENT_ID` | ✅ | mixi2 Developer ConsoleのClient ID |
| `CLIENT_SECRET` | ✅ | mixi2 Developer ConsoleのClient Secret |
| `ADMIN_USER_ID` | ✅ | 勝敗記録を行う管理者のmixi2ユーザーID |
| `TOKEN_URL` | - | トークンエンドポイント（デフォルト値あり） |
| `API_ADDRESS` | - | APIサーバーアドレス（デフォルト値あり） |
| `STREAM_ADDRESS` | - | Streamサーバーアドレス（gRPCモード時） |
| `SIGNATURE_PUBLIC_KEY` | Webhook時 | Ed25519公開鍵（Base64） |
| `PORT` | - | Webhookポート（デフォルト: 8080） |
| `DB_PATH` | - | SQLiteデータベースパス |

### 3. 実行方法

#### ローカル開発（gRPC Streamモード・推奨）

外部公開URLが不要でローカル開発に最適:

```bash
# 依存関係インストール (SQLite CGO)
# Ubuntuの場合:
sudo apt-get install gcc

# ビルド
go build -o bin/stream ./cmd/stream

# 実行
./bin/stream
```

#### 本番（Webhookモード）

HTTPS公開URLが必要:

```bash
go build -o bin/webhook ./cmd/webhook
./bin/webhook
```

Webhookエンドポイント:
- イベント受信: `POST https://YOUR_HOST/events`
- ヘルスチェック: `GET https://YOUR_HOST/healthz`

#### Docker Composeで起動

```bash
# Webhookモード
docker compose up openskill-webhook

# gRPC Streamモード
docker compose --profile stream up openskill-stream
```

### 4. mixi2 Developer ConsoleでWebhook URL登録

1. [Developer Console](https://developer.mixi.social/) にログイン
2. アプリケーション設定 > Webhook設定
3. URLを `https://YOUR_HOST/events` に設定
4. 「接続確認を実行」をクリック
5. 正常に確認できたら完了

## アーキテクチャ

```
openskill-rating-bot/
├── cmd/
│   ├── stream/main.go    # gRPC Streamモードエントリポイント
│   └── webhook/main.go   # Webhookモードエントリポイント
├── internal/
│   ├── config/           # 環境変数管理
│   ├── db/               # SQLiteデータベース操作
│   ├── handler/          # mixi2イベントハンドラ・コマンド処理
│   └── rating/           # OpenSkill Plackett-Luceアルゴリズム
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

## 技術スタック

- **言語**: Go 1.24+
- **mixi2 SDK**: [mixi2-application-sdk-go](https://github.com/mixigroup/mixi2-application-sdk-go)
- **レーティング**: OpenSkill Plackett-Luce (Weng-Lin 2011)
- **データベース**: SQLite (go-sqlite3)
- **接続方式**: gRPC Stream または HTTP Webhook

## ライセンス

MIT License
