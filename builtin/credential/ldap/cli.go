package ldap

import (
	"fmt"
	"os"
	"strings"

	pwd "github.com/hashicorp/go-secure-stdlib/password"
	"github.com/hashicorp/vault/api"
)

type CLIHandler struct{}

func (h *CLIHandler) Auth(c *api.Client, m map[string]string) (*api.Secret, error) {
	mount, ok := m["mount"]
	fmt.Printf("LT - G: mount: %v\n", mount)
	if !ok {
		mount = "ldap"
	}

	username, ok := m["username"]
	fmt.Printf("LT - G: username: %v\n", username)
	if !ok {
		username = usernameFromEnv()
		if username == "" {
			return nil, fmt.Errorf("'username' not supplied and neither 'LOGNAME' nor 'USER' env vars set")
		}
	}
	password, ok := m["password"]
	fmt.Printf("LT - G: password: %v\n", password)
	if !ok {
		fmt.Fprintf(os.Stderr, "Password (will be hidden): ")
		var err error
		password, err = pwd.Read(os.Stdin)
		fmt.Fprintf(os.Stderr, "\n")
		if err != nil {
			return nil, err
		}
	}

	data := map[string]interface{}{
		"password": password,
	}
	fmt.Println("LT - G: data: ", data)

	path := fmt.Sprintf("auth/%s/login/%s", mount, username)
	fmt.Println("LT - G: path: ", path)
	secret, err := c.Logical().Write(path, data)
	fmt.Printf("LT - G: secret: %v\n", secret)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		// TODO: LT This is where we are ending up
		return nil, fmt.Errorf("empty response from credential provider")
	}

	return secret, nil
}

func (h *CLIHandler) Help() string {
	help := `
Usage: vault login -method=ldap [CONFIG K=V...]

  The LDAP auth method allows users to authenticate using LDAP or
  Active Directory.

  Authenticate as "sally":

      $ vault login -method=ldap username=sally
      Password (will be hidden):

  Authenticate as "bob":

      $ vault login -method=ldap username=bob password=password

Configuration:

  password=<string>
      LDAP password to use for authentication. If not provided, the CLI will
      prompt for this on stdin.

  username=<string>
      LDAP username to use for authentication.
`

	return strings.TrimSpace(help)
}

func usernameFromEnv() string {
	if logname := os.Getenv("LOGNAME"); logname != "" {
		return logname
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return ""
}
