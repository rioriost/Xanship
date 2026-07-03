# Xanship

Xanship (transship) は、Docker Desktop で稼働しているコンテナを Apple Container に移行するためのCLIです。

English documentation is available in [README.md](README.md).

ライブマイグレーションではなく、段階的な移行を行います。

1. `docker inspect`、`docker volume ls`、`docker network ls` などで Docker Desktop コンテナをアセスメントする。
2. Apple Container 向けの移行計画を生成する。
3. 必要に応じてイメージを Apple Container にロードする。
4. Apple Container でドライラン作成を行う。
5. Docker の named volume データを Apple Container の volume にコピーする。
6. Docker Desktop 側のコンテナを停止する。
7. Apple Container 側で同等のコンテナを起動する。

## インストール

GitHub Releases から最新版をインストールできます。

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/Xanship/main/scripts/install.sh | sh
```

バージョンを指定してインストールする場合:

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/Xanship/main/scripts/install.sh | sh -s -- 0.1.0
```

インストーラは `/usr/local/bin` に書き込める場合はそこへインストールします。書き込めない場合は `$HOME/.local/bin` にインストールします。別の場所に入れる場合は `INSTALL_DIR` を指定してください。

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/Xanship/main/scripts/install.sh | INSTALL_DIR="$HOME/bin" sh -s -- 0.1.0
```

リリースアーカイブとチェックサムは <https://github.com/rioriost/Xanship/releases> で公開します。

## ビルド

```sh
go build ./cmd/xanship
```

## 使用例

単体の稼働中コンテナをアセスメントする:

```sh
xanship assess --container my-container
xanship commands --dry-run
xanship dry-run --apply
xanship load-images
xanship copy-volumes
xanship stop-docker
xanship start-apple
```

Docker Compose プロジェクトをラベルでアセスメントする:

```sh
xanship assess --compose-project myproject
```

全フェーズを連続実行する:

```sh
xanship migrate --compose-project myproject
```

移行計画は `xanship-plan.json` に `0600` で保存されます。Docker inspect の結果には、環境変数やラベルなど、秘密情報を含む可能性があるためです。

## テスト済み移行

Xanship は代表的な単体コンテナイメージ50件、Docker Compose構成20件で検証済みです。詳細は [docs/tested.md](docs/tested.md) を参照してください。

## 動作確認環境

Xanship 0.1.0 は以下の環境で確認しました。

| コンポーネント | バージョン |
| --- | --- |
| macOS | Apple Silicon 上の 26.5.2 |
| Docker Desktop / Docker Engine | 29.6.1 |
| Apple Container | 1.0.0 |
| container-compose | 1.0.0 |
| Go | 1.22 以降 |

## 現在の対応範囲

Xanship は、image、command、entrypoint、environment、labels、working directory、user、TTY/stdin、init、read-only root filesystem、memory/CPU/shm limits、capabilities、DNS settings、published ports、bind mounts、named volumes、tmpfs mounts、user-defined Docker networks などの一般的な実行時設定を移行します。

一部の Docker 固有の挙動は計画ファイルに警告として出力され、手動確認が必要です。例: restart policy、privileged mode、healthcheck、`extra_hosts`、非標準の mount type、Compose の依存順序。

Apple Container で表現できない Docker label、例えば値に `=` を含むものは、警告付きで移行対象から除外されます。

## ライセンス

Xanship は [MIT License](LICENSE) で公開しています。
