#!/usr/bin/env node
import { execSync } from "node:child_process";
import { mkdirSync, writeFileSync, rmSync, existsSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = __dirname;

const SSH_HOST = "wutuo";
const REMOTE_DIR = "/opt/uploadbroker";
const SVC_NAME = "uploadbroker";

const DEPLOY_DIR = resolve(ROOT, "deploy");
const LOCAL_BIN = resolve(DEPLOY_DIR, SVC_NAME + ".new");
const LOCAL_CONFIG = resolve(DEPLOY_DIR, "uploadBroker.yaml");
const LOCAL_SCRIPT = resolve(DEPLOY_DIR, "deploy.sh");
const TARBALL = resolve(ROOT, "deploy.tar.gz");

function run(cmd) {
  console.log(`\n$ ${cmd}`);
  execSync(cmd, { stdio: "inherit", cwd: ROOT });
}

function step(title, fn) {
  console.log(`\n==> ${title}`);
  fn();
}

// ── 1. 测试 ──
step("1. 运行测试", () => {
  execSync("go test ./... -count=1", { stdio: "inherit", cwd: ROOT });
});

// ── 2. 交叉编译 ──
step("2. 交叉编译 linux/amd64", () => {
  mkdirSync(DEPLOY_DIR, { recursive: true });
  rmSync(resolve(DEPLOY_DIR, SVC_NAME), { force: true });
  execSync(`go build -ldflags "-s -w" -o ${LOCAL_BIN} .`, {
    stdio: "inherit",
    cwd: ROOT,
    env: { ...process.env, GOOS: "linux", GOARCH: "amd64", CGO_ENABLED: "0" },
  });
  console.log(`  -> ${LOCAL_BIN}`);
});

// ── 3. 校验配置 ──
step("3. 校验配置存在", () => {
  if (!existsSync(LOCAL_CONFIG)) {
    throw new Error(`缺少部署配置: ${LOCAL_CONFIG}`);
  }
  console.log(`  ${LOCAL_CONFIG}`);
});

// ── 4. 生成远端部署脚本 ──
step("4. 生成 deploy.sh", () => {
  const script = `#!/bin/bash
set -e
cd ${REMOTE_DIR}

ts() { echo "  [$(date +%s.%N | cut -d. -f1).$(date +%s.%N | cut -d. -f2 | head -c3)] $1"; }

ts "start"
mkdir -p ${REMOTE_DIR}/data/logs
mkdir -p ${REMOTE_DIR}/data/objects

# 生成 systemd 服务文件
cat > /tmp/${SVC_NAME}.service << 'UEOF'
[Unit]
Description=uploadbroker
After=network.target

[Service]
Type=simple
ExecStart=${REMOTE_DIR}/${SVC_NAME} --config ${REMOTE_DIR}/uploadBroker.yaml
WorkingDirectory=${REMOTE_DIR}
Restart=always
RestartSec=3
TimeoutStopSec=10

[Install]
WantedBy=multi-user.target
UEOF

ts "service file"
sudo mv /tmp/${SVC_NAME}.service /etc/systemd/system/${SVC_NAME}.service
sudo systemctl unmask ${SVC_NAME} 2>/dev/null || true
sudo systemctl daemon-reload
ts "daemon-reload"
sudo systemctl enable ${SVC_NAME}
ts "enable"

chmod +x ${SVC_NAME}.new
mv ${SVC_NAME}.new ${SVC_NAME}
ts "replace binary"
sudo systemctl restart ${SVC_NAME}
ts "restart"

sleep 2
if sudo systemctl is-active --quiet ${SVC_NAME}; then
  echo "  ${SVC_NAME}: active"
else
  echo "  ${SVC_NAME}: FAILED"
  journalctl -u ${SVC_NAME} --no-pager -n 15
  exit 1
fi

echo "--- 部署完成，服务运行中 ---"
`;

  writeFileSync(LOCAL_SCRIPT, script, "utf-8");
  console.log("  deploy.sh 已生成");
});

// ── 5. 打包上传 ──
step("5. 打包上传", () => {
  rmSync(TARBALL, { force: true });
  run(`tar czf deploy.tar.gz -C deploy .`);
  run(`scp deploy.tar.gz ${SSH_HOST}:~/deploy.tar.gz`);
});

// ── 6. 执行远端部署 ──
step("6. 执行远端部署", () => {
  run(`ssh ${SSH_HOST} "mkdir -p ${REMOTE_DIR} && mv ~/deploy.tar.gz ${REMOTE_DIR}/ && cd ${REMOTE_DIR} && tar xzf deploy.tar.gz && bash deploy.sh"`);
  rmSync(TARBALL, { force: true });
});

console.log("\n✅ 部署完成");