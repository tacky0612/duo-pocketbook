---
name: docs
description: duo-pocketbook の docs/ 配下のドキュメントを作成・更新・同期する。コード変更に伴うドキュメント更新、ドキュメントの整合性チェック、新規ドキュメントの追加時に使う。
---

# docs — ドキュメント整備スキル

このリポジトリのドキュメントは `docs/` 配下にあり、**実装と常に同期していること**が最重要ルール。このスキルは、ドキュメントの作成・更新・整合性チェックの手順を定める。

## ドキュメント構成と責務

| ファイル | 責務 | 主な同期元 |
|---|---|---|
| `docs/README.md` | 目次・技術スタック概要 | 全体 |
| `docs/architecture.md` | レイヤー構成・依存方向・各層の責務 | `internal/` のパッケージ構成 |
| `docs/settlement.md` | 精算計算の仕様（計算式・端数処理・例・性質） | `internal/domain/settlement.go` |
| `docs/api.md` | 認証フロー・エンドポイント一覧・エラー表 | `api/openapi.yaml`, `internal/web/` |
| `docs/data-model.md` | DynamoDBテーブル設計・アクセスパターン | `internal/infrastructure/dynamodb/` |
| `docs/development.md` | 開発コマンド・テスト戦略・環境変数・CI | `Makefile`, `docker-compose.yml`, `internal/config/`, `.github/workflows/ci.yml` |
| `docs/deployment.md` | デプロイ手順・無料枠の制約・セキュリティ | `terraform/`, `.github/workflows/deploy-pages.yml` |

## 更新手順

1. **変更範囲の特定**: 変更したコード・設定が上表のどの「同期元」に該当するかを確認し、対象ドキュメントを特定する
2. **実装を正とする**: ドキュメントを書く前に必ず実装（コード・Makefile・Terraform・workflow）を読み、実際の挙動・コマンド・値を確認する。記憶や推測で書かない
3. **関連ファイルの連鎖更新**:
   - APIエンドポイントの追加/変更 → `api/openapi.yaml` → `docs/api.md` の順に更新
   - 環境変数の追加 → `internal/config/config.go` を確認して `docs/development.md` の環境変数表を更新
   - AWSリソースの追加 → 無料枠内かを確認したうえで `docs/deployment.md` の無料枠表を更新
   - 精算ロジックの変更 → `docs/settlement.md` の計算式・例を再計算して更新、`README.md` の例も確認
4. **目次の維持**: ドキュメントを追加・削除したら `docs/README.md` の目次と、ルート `README.md` のリンクを更新する

## 記述ルール

- **言語**: 日本語。コード識別子・コマンド・技術用語は原文のまま
- **図は必ず Mermaid 記法で書く**（後述の「図（Mermaid）」を参照）。ASCIIアートや画像ファイルで図を作らない
- **架空を書かない**: 存在しないコマンド・機能・手順を書かない。全コマンドは実行可能であること（不明なら実行して確認する）
- **数値例は検算する**: 精算の具体例は計算式に代入して検算してから記載する
- **表を活用**: エンドポイント・環境変数・無料枠などの列挙は表で書く
- **コードへの参照**: 実装ファイルは `internal/domain/settlement.go` のように相対パスで示す
- **重複より参照**: 同じ内容を複数ファイルに書かず、責務を持つドキュメントへリンクする（例: 精算式の詳細は settlement.md に集約）
- **制約の明示**: AWS無料枠に関わる記述では「なぜその選択か」（例: オンデマンドは無料枠対象外）を必ず併記する

## 図（Mermaid）

ドキュメント内の図はすべて **Mermaid 記法**（` ```mermaid ` フェンス付きコードブロック）で記述する。

記述時の注意:

- ノードラベルに記号・句読点・改行を含める場合は必ず `["..."]` のように**引用符で囲む**。改行は `<br/>`
- ラベル内で括弧 `()` はパース不安定の原因になりやすい。区切りには読点や中黒 `・` を使う（例: `"Lambda<br/>Go, arm64"`）
- 図の種類は内容に合わせて選ぶ: レイヤー/依存関係は `flowchart`、処理の時系列は `sequenceDiagram`、状態遷移は `stateDiagram-v2`
- 矢印の意味（依存の向き・データの流れなど）が曖昧になる場合は、図の直前の文かエッジラベルで明示する

**図を追加・変更したら必ず構文検証する。崩れた Mermaid を残してはならない。**

```bash
make docs-validate          # = node scripts/validate-mermaid.mjs docs
```

`scripts/validate-mermaid.mjs` が `docs/` 配下の全 `mermaid` ブロックを抽出し、`@mermaid-js/mermaid-cli` で1件ずつレンダリング検証する。構文エラーがあれば該当の `ファイル:行` とエラー内容を表示して非ゼロ終了する。CI（`.github/workflows/ci.yml` の `docs-mermaid` ジョブ）でも同じ検証が走るため、ローカルで通してからコミットする。

## 整合性チェック（レビュー観点）

ドキュメント更新後、以下を確認する:

- [ ] 記載コマンドが `Makefile` / 実環境と一致している
- [ ] エンドポイント一覧が `internal/web/router.go` および `api/openapi.yaml` と一致している
- [ ] 環境変数表が `internal/config/config.go` と一致している
- [ ] テーブル設計が `internal/infrastructure/dynamodb/repository.go` のキー定義と一致している
- [ ] Terraformのリソース・変数の説明が `terraform/*.tf` と一致している
- [ ] `docs/README.md` の目次からすべてのドキュメントに到達できる
- [ ] `make docs-validate` が通る（全 Mermaid 図の構文が正しい）
