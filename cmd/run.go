package cmd

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ameerthehacker/zeus/internal/zeus_compiler"
	"github.com/spf13/cobra"
)

func runCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run [file]",
		Short: "Build and run the zeus file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			cwd := getCurDir()
			fileName := filepath.Base(filePath)
			tmpFilePath := filepath.Join(os.TempDir(), fileName+".out")
			objDir := getModeDir(cwd, zeus_compiler.BuildModeDebug, OBJ_DIR_NAME)
			compiler := mustNewCompiler(zeus_compiler.BuildModeDebug)
			// compile the file
			compiler.Compile(filePath, objDir, tmpFilePath)
			// run the file
			runZeusBinCmd := exec.Command(tmpFilePath)
			runZeusBinCmd.Stdin = os.Stdin
			runZeusBinCmd.Stdout = os.Stdout
			runZeusBinCmd.Stderr = os.Stderr
			if err := runZeusBinCmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				os.Exit(1)
			}
		},
	}

	return runCmd
}
