const path = require('path');
const { exec } = require('child_process');
const vscode = require('vscode');

const LANGUAGE_ID = 'oct';

function activate(context) {
  const disposableCommands = [
    vscode.commands.registerCommand('oct.runCurrent', () => runCurrent('run')),
    vscode.commands.registerCommand('oct.testCurrent', () => runCurrent('test')),
    vscode.commands.registerCommand('oct.formatCurrent', () => runCurrent('fmt')),
    vscode.commands.registerCommand('oct.runWorkspace', () => runWorkspace('run')),
    vscode.commands.registerCommand('oct.testWorkspace', () => runWorkspace('test')),
    vscode.commands.registerCommand('oct.formatWorkspace', () => runWorkspace('fmt'))
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

  context.subscriptions.push(...disposableCommands, formatter, taskProvider);
}

function deactivate() {}

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
