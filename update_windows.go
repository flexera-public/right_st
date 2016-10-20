package main

// updateSudoCommand returns the right_st command name with any sudo prefix if necessary. On Windows, it always returns
// just the command name since there is no direct sudo equivalent.
func updateSudoCommand() (string, error) {
	return app.Name, nil
}
