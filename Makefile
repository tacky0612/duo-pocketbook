.PHONY: build test test-integration up down fmt vet lint frontend build-lambda hash clean docs-validate

## バックエンドのビルド
build:
	go build ./...

## ユニットテスト
test:
	go test ./...

## 統合テスト（先に `make up` でローカル環境を起動しておく）
test-integration:
	go test -tags=integration -count=1 ./integration/...

## ローカル環境の起動（アプリ + DynamoDB Local、外部通信なし）
up:
	docker compose up -d --build --wait

## ローカル環境の停止
down:
	docker compose down -v

## フォーマット
fmt:
	gofmt -w .
	terraform -chdir=terraform fmt

## 静的チェック
vet:
	go vet ./...

lint: vet
	@files=$$(gofmt -l .); if [ -n "$$files" ]; then echo "gofmt が必要です:"; echo "$$files"; exit 1; fi

## フロントエンドのビルド
frontend:
	cd frontend && npm install && npm run build

## Lambda デプロイパッケージの作成 (terraform apply の前に実行)
build-lambda:
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o build/bootstrap ./cmd/lambda
	cd build && zip -q lambda.zip bootstrap

## ドキュメント内のMermaid図の構文検証
docs-validate:
	node scripts/validate-mermaid.mjs docs

## パスワードのbcryptハッシュ生成 (例: make hash PASSWORD=secret)
hash:
	go run ./cmd/hashpw '$(PASSWORD)'

clean:
	rm -rf build frontend/dist
