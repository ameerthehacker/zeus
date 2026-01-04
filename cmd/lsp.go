package cmd

import (
	"fmt"
	"os"

	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/lsp"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/spf13/cobra"
)

func lspCmd() *cobra.Command {
	var stdio bool
	
	lspCmd := &cobra.Command{
		Use:   "lsp",
		Short: "Start the Zeus Language Server Protocol server",
		Run: func(cmd *cobra.Command, args []string) {
			// stdio mode is the default behavior
			logger.Log(zeus_error.ErrorSeverityInfo, "Starting Zeus LSP server...")
			
			server := lsp.NewServer()
			if err := server.Start(); err != nil {
				logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("LSP server error: %s", err.Error()))
				os.Exit(1)
			}
		},
	}

	lspCmd.Flags().BoolVar(&stdio, "stdio", true, "Use stdio for communication (default)")

	return lspCmd
}

