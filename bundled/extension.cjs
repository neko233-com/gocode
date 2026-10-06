const vscode = require('vscode');
exports.activate = context => {
  const output = vscode.window.createOutputChannel('Hello Native');
  context.subscriptions.push(output);
  const diagnostics=vscode.languages.createDiagnosticCollection('Hello Native');
  context.subscriptions.push(diagnostics);
  const report=document=>{
    if(document.languageId!=='go')return;
    const items=[];
    for(let i=0;i<document.lineCount;i++){
      const column=document.lineAt(i).text.indexOf('TODO');
      if(column>=0)items.push(new vscode.Diagnostic(new vscode.Range(i,column,i,column+4),'TODO requires attention',vscode.DiagnosticSeverity.Warning));
    }
    diagnostics.set(document.uri,items);
  };
  for(const document of vscode.workspace.textDocuments)report(document);
  context.subscriptions.push(vscode.workspace.onDidOpenTextDocument(report),vscode.workspace.onDidChangeTextDocument(e=>report(e.document)));
  context.subscriptions.push(vscode.languages.registerCompletionItemProvider('go', {
    provideCompletionItems(document, position) {
      const item = new vscode.CompletionItem('nativePrint', vscode.CompletionItemKind.Function);
      item.insertText = 'println("hello from VSIX")';
	  item.detail = 'document-version:'+document.version;
      item.range = document.getWordRangeAtPosition(position) || new vscode.Range(position,position);
      return [item];
    }
  }));
  context.subscriptions.push(vscode.commands.registerCommand('gocode.edit',async()=>{
    const editor=vscode.window.activeTextEditor;
    if(!editor)throw new Error('Open a document first');
    const applied=await editor.edit(edit=>edit.insert(new vscode.Position(0,0),'// VSIX 编辑 😀\n'));
    if(!applied)throw new Error('Document changed before edit');
    output.appendLine('Versioned native edit verified: '+editor.document.version);
    return {applied,version:editor.document.version,text:editor.document.getText()};
  }));
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
