const path = require('path');
const { exec } = require('child_process');
const vscode = require('vscode');

const LANGUAGE_ID = 'oct';
const EN_HUMAN_SETTING = 'enHuman.enabled';
const EN_HUMAN_SCOPE = 'oct.enHuman.enabled';
const EN_HUMAN_REPEAT_MIN = 4;
const SUPERSCRIPT_DIGITS = {
  '0': '⁰',
  '1': '¹',
  '2': '²',
  '3': '³',
  '4': '⁴',
  '5': '⁵',
  '6': '⁶',
  '7': '⁷',
  '8': '⁸',
  '9': '⁹'
};
const DERIVED_UNIT_WHITELIST = new Map([
  ['kg*m/s^2', 'N'],
  ['kg/(m*s^2)', 'Pa'],
  ['kg*m^2/s^2', 'J'],
  ['kg*m^2/s^3', 'W'],
  ['1/s', 'Hz']
]);

function activate(context) {
  const enHumanDecoration = vscode.window.createTextEditorDecorationType({
    opacity: '0.45'
  });
  const enHumanAfterDecoration = vscode.window.createTextEditorDecorationType({
    after: {
      color: new vscode.ThemeColor('editorCodeLens.foreground'),
      margin: '0 0 0 0.35ch'
    },
    rangeBehavior: vscode.DecorationRangeBehavior.ClosedClosed
  });

  const disposableCommands = [
    vscode.commands.registerCommand('oct.runCurrent', () => runCurrent('run')),
    vscode.commands.registerCommand('oct.testCurrent', () => runCurrent('test')),
    vscode.commands.registerCommand('oct.formatCurrent', () => runCurrent('fmt')),
    vscode.commands.registerCommand('oct.runWorkspace', () => runWorkspace('run')),
    vscode.commands.registerCommand('oct.testWorkspace', () => runWorkspace('test')),
    vscode.commands.registerCommand('oct.formatWorkspace', () => runWorkspace('fmt')),
    vscode.commands.registerCommand('oct.toggleEnHuman', async () => {
      const config = vscode.workspace.getConfiguration('oct');
      const enabled = !!config.get(EN_HUMAN_SETTING);
      const next = !enabled;
      await config.update(EN_HUMAN_SETTING, next, vscode.ConfigurationTarget.Global);
      refreshEnHumanForVisibleEditors(enHumanDecoration, enHumanAfterDecoration);
      vscode.window.showInformationMessage(`Oct en-human view ${next ? 'enabled' : 'disabled'}.`);
    })
  ];

  const formatter = vscode.languages.registerDocumentFormattingEditProvider(LANGUAGE_ID, {
    async provideDocumentFormattingEdits(document) {
      if (!document.uri.fsPath) {
        vscode.window.showWarningMessage('Oct: formatting requires a file on disk.');
        return [];
      }

      await document.save();

      const cli = getCliCommand();
      const cwd = workspaceRootForDocument(document) || process.cwd();
      const target = shellQuote(document.uri.fsPath);
      const formatCommand = `${cli} fmt ${target}`;

      try {
        await runCliCommand(formatCommand, cwd);
      } catch (err) {
        const message = formatErrorMessage(err);
        console.error(`Oct format failed: ${message}`);
        vscode.window.showErrorMessage(`Oct format failed: ${message}`);
        return [];
      }

      const refreshed = await vscode.workspace.fs.readFile(document.uri);
      const refreshedText = Buffer.from(refreshed).toString('utf8');
      const fullRange = new vscode.Range(
        document.positionAt(0),
        document.positionAt(document.getText().length)
      );

      return [vscode.TextEdit.replace(fullRange, refreshedText)];
    }
  });

  const taskProvider = vscode.tasks.registerTaskProvider('oct', {
    provideTasks() {
      const folder = firstWorkspaceFolder();
      if (!folder) {
        return [];
      }
      return [
        createWorkspaceTask(folder, 'run', 'Run Workspace'),
        createWorkspaceTask(folder, 'test', 'Test Workspace'),
        createWorkspaceTask(folder, 'fmt', 'Format Workspace')
      ];
    },
    resolveTask() {
      return undefined;
    }
  });

  const docHoverProvider = vscode.languages.registerHoverProvider(LANGUAGE_ID, {
    provideHover(document, position) {
      if (!isEnHumanEnabled()) {
        return undefined;
      }

      const hover = provideDocAwareHover(document, position);
      return hover || undefined;
    }
  });

  const listeners = [
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration(EN_HUMAN_SCOPE)) {
        refreshEnHumanForVisibleEditors(enHumanDecoration, enHumanAfterDecoration);
      }
    }),
    vscode.workspace.onDidChangeTextDocument((event) => {
      if (event.document.languageId !== LANGUAGE_ID) {
        return;
      }
      refreshEnHumanForVisibleEditors(enHumanDecoration, enHumanAfterDecoration, event.document.uri.toString());
    }),
    vscode.window.onDidChangeVisibleTextEditors(() => {
      refreshEnHumanForVisibleEditors(enHumanDecoration, enHumanAfterDecoration);
    }),
    vscode.window.onDidChangeActiveTextEditor(() => {
      refreshEnHumanForVisibleEditors(enHumanDecoration, enHumanAfterDecoration);
    })
  ];

  context.subscriptions.push(
    ...disposableCommands,
    formatter,
    taskProvider,
    docHoverProvider,
    ...listeners,
    enHumanDecoration,
    enHumanAfterDecoration
  );

  refreshEnHumanForVisibleEditors(enHumanDecoration, enHumanAfterDecoration);
}

function deactivate() {}

function refreshEnHumanForVisibleEditors(baseDecoration, afterDecoration, preferredUri) {
  const enabled = isEnHumanEnabled();
  for (const editor of vscode.window.visibleTextEditors) {
    if (editor.document.languageId !== LANGUAGE_ID) {
      continue;
    }

    if (!enabled) {
      editor.setDecorations(baseDecoration, []);
      editor.setDecorations(afterDecoration, []);
      continue;
    }

    if (preferredUri && editor.document.uri.toString() !== preferredUri) {
      continue;
    }

    const replacementDecorations = buildEnHumanDecorations(editor.document);
    const baseRanges = replacementDecorations.map((entry) => entry.range);

    editor.setDecorations(baseDecoration, baseRanges);
    editor.setDecorations(afterDecoration, replacementDecorations);
  }
}

function buildEnHumanDecorations(document) {
  const text = document.getText();
  const used = [];
  const decorations = [];

  const repeatedLiteralDecorations = buildRepeatedLiteralDecorations(document, text, used);
  decorations.push(...repeatedLiteralDecorations);

  for (const [canonical, display] of DERIVED_UNIT_WHITELIST.entries()) {
    const escaped = escapeRegExp(canonical);
    const matcher = new RegExp(`\\b${escaped}\\b`, 'g');
    let match = matcher.exec(text);
    while (match) {
      const start = match.index;
      const end = start + canonical.length;
      if (!intersectsAny(used, start, end)) {
        used.push([start, end]);
        decorations.push(createDecoration(document, start, end, display, canonical, 'derived unit'));
      }
      match = matcher.exec(text);
    }
  }

  const unitPattern = /\b[0-9A-Za-z/*^()]+\b/g;
  let unitMatch = unitPattern.exec(text);
  while (unitMatch) {
    const raw = unitMatch[0];
    const start = unitMatch.index;
    const end = start + raw.length;
    if (!intersectsAny(used, start, end) && isTypographyCandidate(raw)) {
      const rendered = applyUnitTypography(raw);
      if (rendered !== raw) {
        used.push([start, end]);
        decorations.push(createDecoration(document, start, end, rendered, raw, 'unit typography'));
      }
    }
    unitMatch = unitPattern.exec(text);
  }

  const numberUnitPattern = /(?<![A-Za-z0-9_])([+-]?(?:\d+\.\d+|\d+|\.\d+))(?![eE][+-]?\d)([A-Za-z][A-Za-z0-9]*)\b/g;
  let numericMatch = numberUnitPattern.exec(text);
  while (numericMatch) {
    const full = numericMatch[0];
    const numberPart = numericMatch[1];
    const unitPart = numericMatch[2];
    const start = numericMatch.index;
    const end = start + full.length;
    if (intersectsAny(used, start, end)) {
      numericMatch = numberUnitPattern.exec(text);
      continue;
    }

    const displayNumber = toScientificNumber(numberPart);
    if (displayNumber && displayNumber !== numberPart) {
      const rendered = `${displayNumber}${unitPart}`;
      used.push([start, end]);
      decorations.push(createDecoration(document, start, end, rendered, full, 'scientific notation'));
    }
    numericMatch = numberUnitPattern.exec(text);
  }

  return decorations;
}

function createDecoration(document, start, end, display, canonical, transformKind) {
  const canonicalPreview = canonical.length > 220 ? `${canonical.slice(0, 220)}…` : canonical;
  return {
    range: new vscode.Range(document.positionAt(start), document.positionAt(end)),
    hoverMessage: new vscode.MarkdownString(
      `**Oct en-human (${transformKind})**\n\nDisplay-only view: \`${display}\`\n\nCanonical source (edited/saved): \`${canonicalPreview}\``
    ),
    renderOptions: {
      after: {
        contentText: `⟶ ${display}`
      }
    }
  };
}

function createRepeatedLiteralDecoration(document, start, end, display, canonical, value, repeatCount) {
  return {
    range: new vscode.Range(document.positionAt(start), document.positionAt(end)),
    hoverMessage: new vscode.MarkdownString(
      `**Oct en-human (repeated literal compression)**\n\nDisplay-only view: \`${display}\`\n\nCanonical source (edited/saved): \`${canonical}\`\n\nExact literal: \`${value}\`\nRepeat count: ${repeatCount}`
    ),
    renderOptions: {
      after: {
        contentText: `⟶ ${display}`
      }
    }
  };
}

function buildRepeatedLiteralDecorations(document, text, used) {
  const decorations = [];
  const ranges = findTopLevelArrayRanges(text);
  for (const [arrayStart, arrayEnd] of ranges) {
    if (intersectsAny(used, arrayStart, arrayEnd + 1)) {
      continue;
    }

    const rawArray = text.slice(arrayStart, arrayEnd + 1);
    const compression = compressRepeatedScalarArray(rawArray);
    if (!compression) {
      continue;
    }

    used.push([arrayStart, arrayEnd + 1]);
    decorations.push(
      createRepeatedLiteralDecoration(
        document,
        arrayStart,
        arrayEnd + 1,
        compression.display,
        rawArray,
        compression.value,
        compression.repeatCount
      )
    );
  }
  return decorations;
}

function findTopLevelArrayRanges(text) {
  const ranges = [];
  const stack = [];
  let inSingle = false;
  let inDouble = false;
  let escaped = false;

  for (let index = 0; index < text.length; index += 1) {
    const ch = text[index];
    if (inSingle) {
      if (escaped) {
        escaped = false;
      } else if (ch === '\\') {
        escaped = true;
      } else if (ch === '\'') {
        inSingle = false;
      }
      continue;
    }
    if (inDouble) {
      if (escaped) {
        escaped = false;
      } else if (ch === '\\') {
        escaped = true;
      } else if (ch === '"') {
        inDouble = false;
      }
      continue;
    }
    if (ch === '\'') {
      inSingle = true;
      continue;
    }
    if (ch === '"') {
      inDouble = true;
      continue;
    }

    if (ch === '[') {
      stack.push(index);
      continue;
    }
    if (ch === ']' && stack.length > 0) {
      const start = stack.pop();
      if (stack.length === 0) {
        ranges.push([start, index]);
      }
    }
  }

  return ranges;
}

function compressRepeatedScalarArray(rawArray) {
  const trimmed = rawArray.trim();
  if (!trimmed.startsWith('[') || !trimmed.endsWith(']')) {
    return null;
  }

  const inner = trimmed.slice(1, -1).trim();
  if (!inner) {
    return null;
  }

  if (/[()[\]{}]/.test(inner)) {
    return null;
  }

  const parts = inner.split(',').map((part) => part.trim()).filter((part) => part.length > 0);
  if (parts.length < EN_HUMAN_REPEAT_MIN) {
    return null;
  }

  const scalarPattern = /^[+-]?(?:(?:\d+\.\d+)|(?:\d+)|(?:\.\d+))(?:[eE][+-]?\d+)?$/;
  if (!parts.every((part) => scalarPattern.test(part))) {
    return null;
  }

  const first = parts[0];
  if (!parts.every((part) => part === first)) {
    return null;
  }

  return {
    display: `[${first} × ${parts.length}]`,
    value: first,
    repeatCount: parts.length
  };
}

function provideDocAwareHover(document, position) {
  const docs = collectDocBlocks(document.getText());
  if (docs.length === 0) {
    return null;
  }

  const offset = document.offsetAt(position);
  const target = document.getText(document.getWordRangeAtPosition(position, /[A-Za-z_][A-Za-z0-9_]*/));
  if (!target) {
    return null;
  }

  for (const entry of docs) {
    if (offset >= entry.nameStart && offset <= entry.nameEnd) {
      return buildDocHover(entry);
    }
  }

  const declaration = docs.find((entry) => entry.name === target);
  if (!declaration) {
    return null;
  }

  return buildDocHover(declaration);
}

function buildDocHover(entry) {
  const md = new vscode.MarkdownString();
  md.appendMarkdown('**Oct en-human (doc-aware hover)**\n\n');
  md.appendMarkdown(`Declaration: \`${entry.kind} ${entry.name}\`\n\n`);

  if (entry.summary) {
    md.appendMarkdown(`**Summary**\n\n${entry.summary}\n\n`);
  }

  appendDocListSection(md, 'Parameters', entry.params);
  appendDocTextSection(md, 'Returns', entry.returns);
  appendDocTextSection(md, 'Units', entry.units);
  appendDocTextSection(md, 'Remarks', entry.remarks);
  appendDocTextSection(md, 'Example', entry.example);
  md.appendMarkdown('_Display-only enrichment from nearby `///` docs; canonical source remains unchanged._');

  return new vscode.Hover(md);
}

function appendDocListSection(md, label, values) {
  if (!values || values.length === 0) {
    return;
  }
  md.appendMarkdown(`**${label}**\n`);
  for (const value of values) {
    md.appendMarkdown(`- ${value}\n`);
  }
  md.appendMarkdown('\n');
}

function appendDocTextSection(md, label, value) {
  if (!value) {
    return;
  }
  md.appendMarkdown(`**${label}**\n\n${value}\n\n`);
}

function collectDocBlocks(text) {
  const lines = text.split(/\r?\n/);
  const lineOffsets = computeLineOffsets(lines);
  const entries = [];

  for (let lineIndex = 0; lineIndex < lines.length; lineIndex += 1) {
    if (!lines[lineIndex].trimStart().startsWith('///')) {
      continue;
    }

    const docLines = [];
    let cursor = lineIndex;
    while (cursor < lines.length && lines[cursor].trimStart().startsWith('///')) {
      docLines.push(lines[cursor].replace(/^\s*\/\/\/\s?/, ''));
      cursor += 1;
    }

    const declarationLine = lines[cursor] || '';
    const declaration = parseDeclarationLine(declarationLine);
    if (declaration) {
      const parsedDoc = parseStructuredDoc(docLines);
      const lineOffset = lineOffsets[cursor];
      entries.push({
        kind: declaration.kind,
        name: declaration.name,
        nameStart: lineOffset + declaration.nameStart,
        nameEnd: lineOffset + declaration.nameEnd,
        ...parsedDoc
      });
    }

    lineIndex = cursor;
  }

  return entries;
}

function computeLineOffsets(lines) {
  const offsets = [];
  let offset = 0;
  for (const line of lines) {
    offsets.push(offset);
    offset += line.length + 1;
  }
  return offsets;
}

function parseDeclarationLine(line) {
  const declarationPatterns = [
    { kind: 'fn', regex: /^\s*fn\s+([A-Za-z_][A-Za-z0-9_]*)\b/ },
    { kind: 'flow', regex: /^\s*flow\s+([A-Za-z_][A-Za-z0-9_]*)\b/ },
    { kind: 'record', regex: /^\s*record\s+([A-Za-z_][A-Za-z0-9_]*)\b/ },
    { kind: 'enum', regex: /^\s*enum\s+([A-Za-z_][A-Za-z0-9_]*)\b/ }
  ];

  for (const candidate of declarationPatterns) {
    const match = candidate.regex.exec(line);
    if (match && typeof match.index === 'number') {
      const name = match[1];
      const nameStart = match.index + match[0].lastIndexOf(name);
      return {
        kind: candidate.kind,
        name,
        nameStart,
        nameEnd: nameStart + name.length
      };
    }
  }
  return null;
}

function parseStructuredDoc(docLines) {
  const summary = [];
  const params = [];
  const sections = {
    Returns: [],
    Units: [],
    Remarks: [],
    Example: []
  };

  for (const line of docLines) {
    if (!line.trim()) {
      continue;
    }
    const param = /^Param\s+([^:]+):\s*(.+)$/.exec(line);
    if (param) {
      params.push(`\`${param[1].trim()}\`: ${param[2].trim()}`);
      continue;
    }

    const section = /^(Returns|Units|Remarks|Example):\s*(.+)$/.exec(line);
    if (section) {
      sections[section[1]].push(section[2].trim());
      continue;
    }

    summary.push(line.trim());
  }

  return {
    summary: summary.join(' '),
    params,
    returns: sections.Returns.join(' '),
    units: sections.Units.join(' '),
    remarks: sections.Remarks.join(' '),
    example: sections.Example.join(' ')
  };
}

function isTypographyCandidate(token) {
  if (!/[*/^()]/.test(token)) {
    return false;
  }
  if (token.includes('_')) {
    return false;
  }
  return /(kg|m|s|mol|cd|A|K|g)/.test(token);
}

function applyUnitTypography(token) {
  const withMiddleDot = token.replace(/\*/g, '·');
  return withMiddleDot.replace(/\^(\d+)/g, (_, exponent) => exponent.split('').map((digit) => SUPERSCRIPT_DIGITS[digit] || digit).join(''));
}

function toScientificNumber(rawNumber) {
  const numeric = Number(rawNumber);
  if (!Number.isFinite(numeric) || numeric === 0) {
    return null;
  }

  const abs = Math.abs(numeric);
  const shouldConvert = abs >= 1e4 || abs < 1e-3;
  if (!shouldConvert) {
    return null;
  }

  const significantDigits = inferSignificantDigits(rawNumber);
  const precision = Math.max(0, Math.min(12, significantDigits - 1));
  const e = numeric.toExponential(precision);
  return normalizeExponential(e);
}

function inferSignificantDigits(rawNumber) {
  const trimmed = rawNumber.replace(/^[+-]/, '');
  const normalized = trimmed.includes('.')
    ? trimmed.replace('.', '').replace(/^0+/, '').replace(/0+$/, '')
    : trimmed.replace(/^0+/, '').replace(/0+$/, '');

  return normalized.length > 0 ? normalized.length : 1;
}

function normalizeExponential(exponentialText) {
  const [mantissa, exponentRaw] = exponentialText.split('e');
  const exponent = exponentRaw.startsWith('+') || exponentRaw.startsWith('-')
    ? `${exponentRaw[0]}${exponentRaw.slice(1).replace(/^0+/, '') || '0'}`
    : exponentRaw.replace(/^0+/, '') || '0';

  const cleanedMantissa = mantissa.includes('.')
    ? mantissa.replace(/\.0+$/, '').replace(/(\.\d*?)0+$/, '$1')
    : mantissa;

  return `${cleanedMantissa}e${exponent}`;
}

function intersectsAny(ranges, start, end) {
  return ranges.some(([existingStart, existingEnd]) => start < existingEnd && end > existingStart);
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function isEnHumanEnabled() {
  const config = vscode.workspace.getConfiguration('oct');
  return !!config.get(EN_HUMAN_SETTING);
}

function runCurrent(subcommand) {
  const editor = vscode.window.activeTextEditor;
  if (!editor || editor.document.languageId !== LANGUAGE_ID) {
    vscode.window.showWarningMessage('Oct: open an Oct file first.');
    return;
  }

  const target = shellQuote(editor.document.uri.fsPath);
  const cmd = `${getCliCommand()} ${subcommand} ${target}`;
  const cwd = workspaceRootForDocument(editor.document) || path.dirname(editor.document.uri.fsPath);

  runInTerminal(cmd, cwd, true);
}

function runWorkspace(subcommand) {
  const folder = firstWorkspaceFolder();
  if (!folder) {
    vscode.window.showWarningMessage('Oct: open a workspace folder first.');
    return;
  }

  const cmd = `${getCliCommand()} ${subcommand} .`;
  runInTerminal(cmd, folder.uri.fsPath, true);
}

function createWorkspaceTask(folder, subcommand, label) {
  const definition = { type: 'oct', subcommand };
  const cli = getCliCommand();
  const execution = new vscode.ShellExecution(`${cli} ${subcommand} .`, {
    cwd: folder.uri.fsPath
  });

  const problemMatchers = subcommand === 'fmt' ? [] : ['$go'];
  return new vscode.Task(
    definition,
    folder,
    `Oct: ${label}`,
    'oct',
    execution,
    problemMatchers
  );
}

function workspaceRootForDocument(document) {
  const folder = vscode.workspace.getWorkspaceFolder(document.uri);
  return folder ? folder.uri.fsPath : undefined;
}

function firstWorkspaceFolder() {
  return vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders.length > 0
    ? vscode.workspace.workspaceFolders[0]
    : undefined;
}

function getCliCommand() {
  const config = vscode.workspace.getConfiguration('oct');
  const configured = config.get('cli.command');
  return configured && String(configured).trim().length > 0
    ? String(configured).trim()
    : 'go run ./cmd/oct';
}

function runInTerminal(command, cwd, reveal) {
  try {
    const terminal = vscode.window.createTerminal({
      name: 'Oct',
      cwd
    });

    terminal.sendText(command, true);
    if (reveal) {
      terminal.show();
    }
  } catch (err) {
    vscode.window.showErrorMessage(`Oct command failed: ${formatErrorMessage(err)}`);
  }
}

function runCliCommand(command, cwd) {
  return new Promise((resolve, reject) => {
    exec(command, { cwd }, (error, stdout, stderr) => {
      if (error) {
        reject(new Error([stderr, stdout, error.message].filter(Boolean).join('\n').trim()));
        return;
      }
      resolve();
    });
  });
}

function shellQuote(value) {
  if (!value) {
    return "''";
  }
  return `'${value.replace(/'/g, `'\\''`)}'`;
}

function formatErrorMessage(err) {
  if (!err) {
    return 'unknown error';
  }
  if (typeof err === 'string') {
    return err;
  }
  if (err.message) {
    return err.message;
  }
  return String(err);
}

module.exports = {
  activate,
  deactivate
};
