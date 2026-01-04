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

export function activate(context: vscode.ExtensionContext) {
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

    client.start();

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
