# Xanship

Xanship (transship) は、Docker で稼働しているコンテナを macOS 上の Apple Container に移行するためのCLIです。

English documentation is available in [README.md](README.md).

ライブマイグレーションではなく、段階的な移行を行います。

1. `docker inspect`、`docker volume ls`、`docker network ls` などで Docker コンテナをアセスメントする。
2. Apple Container 向けの移行計画を生成する。
3. 必要に応じてイメージを Apple Container にロードする。
4. Apple Container でドライラン作成を行う。
5. Docker の named volume データを Apple Container の volume にコピーする。
6. Docker Desktop 側のコンテナを停止する。
7. Apple Container 側で同等のコンテナを起動する。

## 移行イメージ

<table>
  <tr>
    <td align="center"><strong>Docker Desktop 上</strong></td>
    <td align="center" rowspan="2"><h1>→</h1></td>
    <td align="center"><strong>Apple Container 上</strong></td>
  </tr>
  <tr>
    <td align="center"><img src="images/on_docker.png" alt="Docker Desktop 上で稼働するコンテナ" width="360"></td>
    <td align="center"><img src="images/on_ac.png" alt="Apple Container に移行されたコンテナ" width="360"></td>
  </tr>
</table>

## インストール

GitHub Releases から最新版をインストールできます。

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/xanship/main/scripts/install.sh | sh
```

バージョンを指定してインストールする場合:

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/xanship/main/scripts/install.sh | sh -s -- 0.3.0
```

インストーラは `/usr/local/bin` に書き込める場合はそこへインストールします。書き込めない場合は `$HOME/.local/bin` にインストールします。別の場所に入れる場合は `INSTALL_DIR` を指定してください。

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/xanship/main/scripts/install.sh | INSTALL_DIR="$HOME/bin" sh -s -- 0.3.0
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

互換性の事前確認と移行レポート生成:

```sh
xanship preflight --plan xanship-plan.json
xanship report --plan xanship-plan.json > xanship-report.md
```

より安全な移行制御:

```sh
xanship assess --container my-container --bind-policy copy-to-volume --existing reuse
xanship copy-volumes --verify
xanship rollback --plan xanship-plan.json
```

Docker Compose プロジェクトをラベルでアセスメントする:

```sh
xanship assess --compose-project myproject --service web --exclude-service debug
```

全フェーズを連続実行する:

```sh
xanship migrate --compose-project myproject
```

SSH越しのLinuxホスト上で動くDockerから、ローカルのApple Containerへ移行する:

```sh
xanship assess --docker-host ssh://ubuntu@linux-host --container web
xanship load-images --docker-host ssh://ubuntu@linux-host
xanship copy-volumes --docker-host ssh://ubuntu@linux-host --verify
xanship stop-docker --docker-host ssh://ubuntu@linux-host
xanship start-apple
```

Docker CLI contextがLinux sourceを向いている場合は、`--docker-context CONTEXT` も利用できます。

移行計画は `xanship-plan.json` に `0600` で保存されます。Docker inspect の結果には、環境変数やラベルなど、秘密情報を含む可能性があるためです。

## 安全性と運用機能

Xanship は、段階的な切り替えのための移行制御を備えています。

| 機能 | コマンドまたはオプション |
| --- | --- |
| 互換性の事前確認とポート競合チェック | `xanship preflight --plan xanship-plan.json` |
| Markdown移行レポート | `xanship report --plan xanship-plan.json` |
| 切り替え失敗後のロールバック | `xanship rollback --plan xanship-plan.json` |
| Appleリソースの冪等な扱い | <code>--existing fail&#124;reuse&#124;replace</code> |
| named volumeコピーの検証 | `xanship copy-volumes --verify` / `xanship verify-volumes` |
| bind mountの扱いの制御 | <code>--bind-policy keep&#124;warn&#124;fail&#124;copy-to-volume</code> |
| 計画ファイルの確認と編集 | `xanship plan summary`, `xanship plan validate`, `xanship plan set` |
| Compose serviceの絞り込みと順序制御 | `--service`, `--exclude-service`, `depends_on`順 |
| イメージ転送の堅牢化 | Apple Container pullを優先し、Docker save/loadへフォールバック |

## Linux Dockerソース

macOS上のローカルDocker Desktopに加えて、Docker CLIのSSH transportを使い、Linuxホスト上のDockerコンテナをアセスメント・停止できます。

```sh
xanship assess --docker-host ssh://user@linux-host --container web
xanship load-images --docker-host ssh://user@linux-host
xanship copy-volumes --docker-host ssh://user@linux-host --verify
xanship stop-docker --docker-host ssh://user@linux-host
xanship start-apple
```

Linux Dockerホスト上の named volume データは、ローカルの Apple Container volume へストリームコピーされます。ソースホストが x86_64 の場合は、Apple Silicon互換のvariantを持つイメージを利用してください。

## テスト済み移行

Xanship は代表的な単体コンテナイメージ50件、Docker Compose構成20件で検証済みです。詳細は [docs/tested.md](docs/tested.md) を参照してください。

追加のエンドツーエンド検証:

| ソース | アーキテクチャ | Dockerバージョン | シナリオ | 結果 |
| --- | --- | --- | --- | --- |
| Docker Desktop on macOS | arm64 | 29.6.1 | 代表的な単体コンテナとComposeプロジェクト | 成功 |
| Parallels Desktop上のUbuntu Linux | arm64 | 29.1.3 | SSH Dockerソース、named volume付きnginx | 成功 |
| CentOS Stream 8 Linux | x86_64 | 26.1.3 | SSH Dockerソース、`nginx:alpine` + named volume、Apple Silicon上のApple Containerへ移行 | 成功 |

## 動作確認環境

Xanship 0.3.0 は以下の環境で確認しました。

| コンポーネント | バージョン |
| --- | --- |
| macOS | Apple Silicon 上の 26.5.2 |
| Docker Desktop / Docker Engine | 29.6.1 |
| Apple Container | 1.0.0 |
| container-compose | 1.0.0 |
| Go | 1.22 以降 |
| リモートLinux Dockerソース | Parallels Desktop上のUbuntu arm64、SSH越しのCentOS Stream 8 x86_64 |

## 現在の対応範囲

Xanship は、image、command、entrypoint、environment、labels、working directory、user、TTY/stdin、init、read-only root filesystem、memory/CPU/shm limits、capabilities、DNS settings、published ports、bind mounts、named volumes、tmpfs mounts、user-defined Docker networks、Compose serviceの選択と順序などの一般的な実行時設定を移行します。

一部の Docker 固有の挙動は計画ファイルに警告として出力され、手動確認が必要です。例: restart policy、privileged mode、healthcheck、`extra_hosts`、非標準の mount type。

Apple Container で表現できない Docker label、例えば値に `=` を含むものは、警告付きで移行対象から除外されます。

## リリースゲート

リリース前に以下を実行してください。

```sh
make release-check
```

リリースゲートでは、フォーマット、テスト、`go vet`、インストーラ構文、バージョン埋め込み、対応プラットフォーム向けリリースアーカイブ生成を確認します。

## ライセンス

Xanship は [MIT License](LICENSE) で公開しています。
