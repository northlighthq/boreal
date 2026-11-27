package open

import "os/exec"

func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	go cmd.Wait()
	return nil
}
