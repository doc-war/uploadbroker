#!/usr/bin/env node
import { execSync } from "node:child_process";
import { copyFileSync, mkdirSync, writeFileSync, existsSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = __dirname;

const SSH_HOST = "wutuo";
const REMOTE_DIR = "/opt/uploadbroker";
const SVC_NAME = "uploadbroker";
const FORCE_CONFIG = process.argv.includes("--force-config");

const DEPLOY_DIR = resolve(ROOT, "deploy");
const LOCAL_BIN = resolve(DEPLOY_DIR, SVC_NAME);
const LOCAL_CONFIG = resolve(ROOT, "uploadBroker.yaml");
const DEPLOY_CONFIG = resolve(DEPLOY_DIR, "uploadBroker.yaml");
const LOCAL_SCRIPT = resolve(DEPLOY_DIR, "deploy.sh");

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
  execSync(`go build -ldflags "-s -w" -o ${LOCAL_BIN} .`, {
    stdio: "inherit",
    cwd: ROOT,
    env: { ...process.env, GOOS: "linux", GOARCH: "amd64", CGO_ENABLED: "0" },
  });
  console.log(`  -> ${LOCAL_BIN}`);
});

// ── 3. 拷贝配置 ──
step("3. 拷贝配置到 deploy 目录", () => {
  copyFileSync(LOCAL_CONFIG, DEPLOY_CONFIG);
  console.log("  uploadBroker.yaml -> deploy/uploadBroker.yaml");
});

// ── 4. 生成远端部署脚本 ──
step("4. 生成 deploy.sh", () => {
  const script = `#!/bin/bash
set -e
cd ${REMOTE_DIR}

# 创建数据目录
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

[Install]
WantedBy=multi-user.target
UEOF

sudo mv /tmp/${SVC_NAME}.service /etc/systemd/system/${SVC_NAME}.service
sudo systemctl unmask ${SVC_NAME} 2>/dev/null || true
sudo systemctl daemon-reload
sudo systemctl enable ${SVC_NAME}
sudo systemctl stop ${SVC_NAME} || true

chmod +x ${SVC_NAME}.new
mv ${SVC_NAME}.new ${SVC_NAME}
sudo systemctl start ${SVC_NAME}

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

// ── 5. 创建远端目录 ──
step("5. 创建远端目录", () => {
  run(`ssh ${SSH_HOST} "mkdir -p ${REMOTE_DIR}"`);
});

// ── 6. 上传文件 ──
step("6. 上传文件", () => {
  run(`scp ${LOCAL_BIN} ${SSH_HOST}:${REMOTE_DIR}/${SVC_NAME}.new`);
  run(`scp ${LOCAL_SCRIPT} ${SSH_HOST}:${REMOTE_DIR}/`);

  if (FORCE_CONFIG) {
    run(`scp ${DEPLOY_CONFIG} ${SSH_HOST}:${REMOTE_DIR}/uploadBroker.yaml`);
    console.log("  配置已强制覆盖");
  } else {
    const remoteExists = (() => {
      try {
        execSync(`ssh ${SSH_HOST} "test -f ${REMOTE_DIR}/uploadBroker.yaml"`, {
          stdio: "pipe",
        });
        return true;
      } catch {
        return false;
      }
    })();

    if (!remoteExists) {
      run(`scp ${DEPLOY_CONFIG} ${SSH_HOST}:${REMOTE_DIR}/uploadBroker.yaml`);
      console.log("  配置已上传（远端首次）");
    } else {
      console.log("  跳过配置上传（远端已存在，如需覆盖请加 --force-config）");
    }
  }
});

// ── 7. 执行远端部署 ──
step("7. 执行远端部署脚本", () => {
  run(`ssh ${SSH_HOST} "bash ${REMOTE_DIR}/deploy.sh"`);
});

console.log("\n✅ 部署完成");
