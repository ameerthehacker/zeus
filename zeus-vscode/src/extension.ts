import * as vscode from "vscode";
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

function getZeusExecutable(): string {
    const config = vscode.workspace.getConfiguration("zeus");
    const configuredPath = config.get<string>("executablePath");

    if (configuredPath && configuredPath.trim() !== "") {
        return configuredPath;
    }

    return "zeus";
}

async function startLanguageServer(): Promise<void> {
    const zeusExecutable = getZeusExecutable();

    const serverOptions: ServerOptions = {
        run: {
            command: zeusExecutable,
            args: ["lsp", "--stdio"],
            transport: TransportKind.stdio,
        },
        debug: {
            command: zeusExecutable,
            args: ["lsp", "--stdio"],
            transport: TransportKind.stdio,
            options: {
                env: {
                    ...process.env,
                    ZEUS_LSP_DEBUG: "1",
                },
            },
        },
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [{ scheme: "file", language: "zeus" }],
        synchronize: {
            fileEvents: vscode.workspace.createFileSystemWatcher("**/*.zs"),
        },
    };

    client = new LanguageClient(
        "zeusLanguageServer",
        "Zeus Language Server",
        serverOptions,
        clientOptions
    );

    await client.start();
}

async function stopLanguageServer(): Promise<void> {
    if (client) {
        await client.stop();
        client = undefined;
    }
}

async function restartLanguageServer(): Promise<void> {
    await stopLanguageServer();
    await startLanguageServer();
    vscode.window.showInformationMessage("Zeus Language Server restarted");
}

export function activate(context: vscode.ExtensionContext) {
    // Start the language server
    startLanguageServer();

    // Register restart command
    const restartCommand = vscode.commands.registerCommand(
        "zeus.restartLanguageServer",
        restartLanguageServer
    );

    context.subscriptions.push(restartCommand);

    context.subscriptions.push({
        dispose: () => {
            if (client) {
                client.stop();
            }
        },
    });
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
