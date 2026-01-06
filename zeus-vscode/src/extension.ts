import * as vscode from "vscode";
import * as fs from "fs";
import * as os from "os";
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

/**
 * Get the Zeus executable path.
 * Search order:
 * 1. Homebrew installation paths (macOS)
 *    - /opt/homebrew/bin/zeus (Apple Silicon)
 *    - /usr/local/bin/zeus (Intel)
 * 2. Custom path from zeus.executablePath setting
 * 3. 'zeus' from system PATH
 */
function getZeusExecutable(): string {
    // Check Homebrew paths first (macOS)
    if (os.platform() === "darwin") {
        const homebrewPaths = [
            "/opt/homebrew/bin/zeus", // Apple Silicon
            "/usr/local/bin/zeus", // Intel
        ];

        for (const brewPath of homebrewPaths) {
            if (fs.existsSync(brewPath)) {
                return brewPath;
            }
        }
    }

    // Check user-configured path
    const config = vscode.workspace.getConfiguration("zeus");
    const configuredPath = config.get<string>("executablePath");

    if (configuredPath && configuredPath.trim() !== "") {
        return configuredPath;
    }

    // Fallback to PATH
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
        try {
            await client.stop();
        } catch {
            // Ignore errors when stopping - server might not be running
        }
        client = undefined;
    }
}

async function restartLanguageServer(): Promise<void> {
    const wasRunning = client !== undefined;
    await stopLanguageServer();

    try {
        await startLanguageServer();
        vscode.window.showInformationMessage(
            wasRunning
                ? "Zeus Language Server restarted"
                : "Zeus Language Server started"
        );
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        vscode.window.showErrorMessage(
            `Failed to start Zeus Language Server: ${message}`
        );
    }
}

export function activate(context: vscode.ExtensionContext) {
    // Start the language server
    startLanguageServer().catch((error) => {
        const message = error instanceof Error ? error.message : String(error);
        vscode.window.showErrorMessage(
            `Failed to start Zeus Language Server: ${message}`
        );
    });

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
