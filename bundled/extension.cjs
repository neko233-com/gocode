const vscode = require('vscode');
exports.activate = context => {
  const output = vscode.window.createOutputChannel('Hello Native');
  context.subscriptions.push(output);
  context.subscriptions.push(vscode.commands.registerCommand('gocode.hello', async () => {
    output.appendLine('Hello from a VSIX extension running on Node.js.');
    output.show();
    await vscode.window.showInformationMessage('Hello from the native Go workbench!');
    const status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left);
    status.text = 'Hello Native'; status.show(); context.subscriptions.push(status);
    return 'hello-native';
  }));
  context.subscriptions.push(vscode.commands.registerCommand('gocode.readme', async () => {
    const uri = vscode.Uri.joinPath(vscode.workspace.workspaceFolders[0].uri, 'README.md');
    const document = await vscode.workspace.openTextDocument(uri);
    await vscode.window.showTextDocument(document);
  }));
};
