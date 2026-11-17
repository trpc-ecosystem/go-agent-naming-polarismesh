[English](README.md) | 中文

# AI Registry 远端 E2E

`cloud/` 用于构建可在另一台主机上运行 AI Registry 真实 E2E 的自包含部署包。
包内包含预编译的 Provider、Consumer、E2E、cleanup 二进制，以及现有的
`verify_all.sh`、结果摘要、日志回传和所有权保护的清理工具。

目标主机不需要安装 Go，也不需要访问项目源码树。

## 构建

在仓库根目录执行：

```bash
./examples/ai-registry/cloud/build-package.sh
```

默认输出：

```text
examples/ai-registry/cloud/dist/go-agent-naming-polarismesh-cloud-<version>-<commit>-<time>.tar.gz
examples/ai-registry/cloud/dist/go-agent-naming-polarismesh-cloud-<version>-<commit>-<time>.tar.gz.sha256
```

默认目标为 Linux/amd64。构建其他架构：

```bash
./examples/ai-registry/cloud/build-package.sh \
  --goos linux \
  --goarch arm64
```

可使用 `--name` 和 `--output-dir` 修改包名或输出目录，使用 `--clean` 清理
本地 cloud 构建产物。

部署包使用 `CGO_ENABLED=0` 和裁剪后的二进制。`manifest.json` 记录适配层版本、
Git commit、工作区是否 dirty、构建时间、目标平台和 Go 版本。

## 上传

```bash
scp examples/ai-registry/cloud/dist/<package>.tar.gz \
    examples/ai-registry/cloud/dist/<package>.tar.gz.sha256 \
    <user>@<cloud-host>:~/
```

远端主机：

```bash
sha256sum -c <package>.tar.gz.sha256
tar xzf <package>.tar.gz
cd <package>
```

没有 GNU `sha256sum` 时使用 `shasum -a 256 -c`。

## 运行

远端主机需要 `bash`、`tar`、`jq`、`ps`、`curl` 和常规进程检查工具。预编译包
不要求安装 Go。

保留资源：

```bash
POLARIS_TOKEN=<optional> \
POLARIS_SERVICE_TOKEN=<optional> \
./run-e2e.sh \
  --server <polaris-host> \
  --resources keep
```

自动执行所有权保护的清理：

```bash
./run-e2e.sh \
  --server <polaris-host> \
  --resources cleanup \
  --run-id review-run
```

自定义端口和地址：

```bash
./run-e2e.sh \
  --server <polaris-host> \
  --discover <polaris-host>:8091 \
  --console http://<polaris-host>:8090 \
  --skill-address <polaris-host>:8094 \
  --namespace go-agent-naming-e2e \
  --timeout 10s \
  --run-timeout 20m
```

默认 `keep` 策略保留 state 所属资源和最后一个 Provider。需要本轮自动删除实例、
服务和 Skill 时使用 `cleanup`。清理会校验 owner、run ID、PID、进程启动时间、
完整命令和真实实例状态。

凭据只从以下环境变量读取：

- `POLARIS_TOKEN`
- `POLARIS_CONSOLE_TOKEN`
- `POLARIS_SKILL_TOKEN`
- `POLARIS_SERVICE_TOKEN`

token 不会写入 `environment.txt`、manifest、state 或结果摘要。

## 运行目录

```text
cloud-runs/<run-id>/
├── analysis.md
├── environment.txt
├── manifest.json
├── results.json
├── run.log
├── state.json
└── providers/
```

用例执行时还可能生成服务和 Skill 的只读投影诊断文件。

## 日志

`run-e2e.sh` 默认生成带校验文件的日志包：

```text
cloud-runs/<run-id>-logs.tar.gz
cloud-runs/<run-id>-logs.tar.gz.sha256
```

手动打包：

```bash
./pack-logs.sh --run-dir cloud-runs/<run-id>
./pack-logs.sh --all
```

回传并导入本地：

```bash
scp <user>@<cloud-host>:~/cloud-runs/<run-id>-logs.tar.gz .
scp <user>@<cloud-host>:~/cloud-runs/<run-id>-logs.tar.gz.sha256 .

./examples/ai-registry/cloud/import-logs.sh ./<run-id>-logs.tar.gz
```

导入脚本会校验相邻 checksum，拒绝绝对路径、父目录穿越和归档链接，并生成
`analysis.md`。

## 清理

预览：

```bash
./cleanup-run.sh --run-id <run-id> --dry-run
```

执行：

```bash
./cleanup-run.sh \
  --run-id <run-id> \
  --apply \
  --delete-service
```

清理范围只来自运行 state 和现有 ownership 规则选中的资源。

## 安全

- 不要把 token 放入 URL 或命令行参数。
- 不要在 state 或 results 中保存带凭据的 URL。
- 远端日志可能包含服务名、实例 ID、interface URL 和请求诊断，应按内部数据处理。
- 不要绕过 ownership 和进程身份校验，按宽泛名称删除进程或资源。
