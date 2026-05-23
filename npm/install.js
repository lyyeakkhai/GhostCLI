#!/usr/bin/env node
/**
 * GhostCLI NPM Wrapper — install script
 *
 * Downloads the correct GhostCLI binary for the current platform/architecture
 * from GitHub Releases, verifies its SHA-256 checksum, and places it in the
 * expected location so the package bin scripts work out of the box.
 */

const fs = require('fs');
const https = require('https');
const path = require('path');
const { spawn } = require('child_process');
const crypto = require('crypto');

const PACKAGE_NAME = '@ghostcli/proxy';
const GITHUB_REPO = 'ghostcli/ghostcli';
const BINARY_NAME = 'ghost';

// Map Node process.platform to release artifact OS names
const PLATFORM_MAP = {
  darwin: 'darwin',
  linux: 'linux',
  win32: 'windows',
};

// Map Node process.arch to release artifact arch names
const ARCH_MAP = {
  x64: 'amd64',
  arm64: 'arm64',
};

function getTarget() {
  const platform = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];

  if (!platform || !arch) {
    throw new Error(
      `Unsupported platform: ${process.platform}/${process.arch}. ` +
        'GhostCLI supports Windows amd64, macOS amd64/arm64, and Linux amd64/arm64.'
    );
  }

  const ext = platform === 'windows' ? '.exe' : '';
  return { platform, arch, ext };
}

function getPackageVersion() {
  const pkgPath = path.join(__dirname, 'package.json');
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
  return pkg.version;
}

function getBinaryDir() {
  // When installed as a dependency, __dirname is inside node_modules/@ghostcli/proxy/
  return path.join(__dirname, 'bin');
}

function getBinaryPath() {
  const { ext } = getTarget();
  return path.join(getBinaryDir(), `${BINARY_NAME}${ext}`);
}

function download(url) {
  return new Promise((resolve, reject) => {
    const req = https.get(url, { redirect: 'follow' }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return resolve(download(res.headers.location));
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`HTTP ${res.statusCode}: ${url}`));
      }

      const chunks = [];
      res.on('data', (chunk) => chunks.push(chunk));
      res.on('end', () => resolve(Buffer.concat(chunks)));
    });

    req.on('error', reject);
    req.setTimeout(30000, () => {
      req.destroy();
      reject(new Error('Download timeout'));
    });
  });
}

async function fetchChecksum(version, platform, arch) {
  const checksumUrl = `https://github.com/${GITHUB_REPO}/releases/download/v${version}/ghost-${platform}-${arch}.sha256`;
  try {
    const data = await download(checksumUrl);
    const text = data.toString('utf8').trim();
    // Format: "<hash>  ghost-<os>-<arch>"
    const parts = text.split(/\s+/);
    return parts[0];
  } catch (err) {
    console.warn(`Warning: could not fetch checksum: ${err.message}`);
    return null;
  }
}

function verifyChecksum(filePath, expectedHash) {
  if (!expectedHash) return true;

  const hash = crypto.createHash('sha256');
  hash.update(fs.readFileSync(filePath));
  const actualHash = hash.digest('hex');

  if (actualHash !== expectedHash) {
    throw new Error(
      `Checksum mismatch! Expected ${expectedHash}, got ${actualHash}. ` +
        'The downloaded binary may be corrupted or tampered with.'
    );
  }

  return true;
}

async function install() {
  const { platform, arch, ext } = getTarget();
  const version = getPackageVersion();
  const binaryDir = getBinaryDir();
  const binaryPath = getBinaryPath();

  // Skip if binary already exists (e.g., manual install or cached)
  if (fs.existsSync(binaryPath)) {
    console.log(`GhostCLI binary already exists at ${binaryPath}`);
    return;
  }

  if (!fs.existsSync(binaryDir)) {
    fs.mkdirSync(binaryDir, { recursive: true });
  }

  const artifactName = `ghost-${platform}-${arch}${ext}`;
  const downloadUrl = `https://github.com/${GITHUB_REPO}/releases/download/v${version}/${artifactName}`;

  console.log(`Downloading GhostCLI v${version} for ${platform}/${arch} ...`);
  console.log(`  URL: ${downloadUrl}`);

  const binaryData = await download(downloadUrl);
  fs.writeFileSync(binaryPath, binaryData);

  // Make executable on Unix
  if (process.platform !== 'win32') {
    fs.chmodSync(binaryPath, 0o755);
  }

  // Verify checksum
  const expectedHash = await fetchChecksum(version, platform, arch);
  if (expectedHash) {
    verifyChecksum(binaryPath, expectedHash);
    console.log('Checksum verified.');
  }

  console.log(`GhostCLI v${version} installed at ${binaryPath}`);
}

function uninstall() {
  const binaryPath = getBinaryPath();
  if (fs.existsSync(binaryPath)) {
    fs.unlinkSync(binaryPath);
    console.log(`Removed ${binaryPath}`);
  }
}

// ─── Main ───────────────────────────────────────────────────────────────────

(async () => {
  try {
    const args = process.argv.slice(2);

    if (args.includes('--uninstall')) {
      uninstall();
    } else {
      await install();
    }
  } catch (err) {
    console.error(`Error: ${err.message}`);

    // Provide fallback instructions
    console.error('\nYou can manually install GhostCLI by:');
    console.error('  1. Downloading the binary from https://github.com/ghostcli/ghostcli/releases');
    console.error('  2. Placing it in your PATH as "ghost"');
    console.error('  3. Or run: go install github.com/ghostcli/ghostcli/cmd/ghost@latest');

    process.exit(1);
  }
})();
