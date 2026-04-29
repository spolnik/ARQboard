import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const outputDir = path.join(root, 'coverage');

const thresholds = {
  goStatements: numberFromEnv('GO_COVERAGE_MIN', 75),
  webLines: numberFromEnv('WEB_LINES_COVERAGE_MIN', 85),
  webStatements: numberFromEnv('WEB_STATEMENTS_COVERAGE_MIN', 84),
  webBranches: numberFromEnv('WEB_BRANCHES_COVERAGE_MIN', 75),
};

const goStatements = readGoStatements();
const web = readWebCoverage();

const checks = [
  ['Go statements', goStatements, thresholds.goStatements],
  ['Web lines', web.lines, thresholds.webLines],
  ['Web statements', web.statements, thresholds.webStatements],
  ['Web branches', web.branches, thresholds.webBranches],
];

const failures = checks.filter(([, actual, minimum]) => actual < minimum);

for (const [name, actual, minimum] of checks) {
  console.log(`${name}: ${actual.toFixed(1)}% (minimum ${minimum.toFixed(1)}%)`);
}

if (failures.length > 0) {
  console.error('\nCoverage is below the required minimum:');
  for (const [name, actual, minimum] of failures) {
    console.error(`- ${name}: ${actual.toFixed(1)}% < ${minimum.toFixed(1)}%`);
  }
  process.exit(1);
}

function readGoStatements() {
  const summaryPath = path.join(outputDir, 'go-summary.txt');
  const totalLine = readText(summaryPath)
    .split(/\r?\n/)
    .find((line) => line.trim().startsWith('total:'));

  if (!totalLine) {
    throw new Error(`Could not find Go total coverage in ${summaryPath}`);
  }

  const match = totalLine.match(/([\d.]+)%/);
  if (!match) {
    throw new Error(`Could not parse Go total coverage from ${totalLine}`);
  }

  return Number(match[1]);
}

function readWebCoverage() {
  const summaryPath = path.join(root, 'web', 'coverage', 'coverage-summary.json');
  const summary = JSON.parse(readText(summaryPath));
  const total = summary.total;

  return {
    lines: total.lines.pct,
    statements: total.statements.pct,
    branches: total.branches.pct,
  };
}

function numberFromEnv(name, fallback) {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const value = Number(raw);
  if (!Number.isFinite(value)) {
    throw new Error(`${name} must be numeric`);
  }
  return value;
}

function readText(filePath) {
  const buffer = fs.readFileSync(filePath);
  const utf8 = buffer.toString('utf8');
  if (utf8.includes('\u0000')) {
    return buffer.toString('utf16le');
  }
  return utf8;
}
