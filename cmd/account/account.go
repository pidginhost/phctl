package account

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var (
	outputFormat = cmdutil.OutputFormat
	force        = cmdutil.Force
)

var Cmd = &cobra.Command{
	Use:   "account",
	Short: "Manage account, profile, SSH keys, and companies",
}

// --- Profile ---

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "View account profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		profile, _, err := c.AccountAPI.AccountProfileRetrieve(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("getting profile: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, profile, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "First Name:", profile.FirstName)
			output.PrintRow(tw, "Last Name:", profile.LastName)
			output.PrintRow(tw, "Funds:", profile.Funds)
			output.PrintRow(tw, "Phone:", profile.Phone)
			tw.Flush()
		})
		return nil
	},
}

// --- SSH Keys ---

var sshKeyCmd = &cobra.Command{
	Use:     "ssh-key",
	Aliases: []string{"ssh"},
	Short:   "Manage SSH keys",
}

var sshKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SSH keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.AccountAPI.AccountSshKeysList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing SSH keys: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ALIAS", "FINGERPRINT")
			for _, k := range resp.Results {
				output.PrintRow(tw, k.Id, pstr(k.Alias), k.Fingerprint)
			}
			tw.Flush()
		})
		return nil
	},
}

var (
	sshKeyCreateAlias string
	sshKeyCreateKey   string
)

var sshKeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Add an SSH key",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewSSHKey(0, "", sshKeyCreateKey)
		if sshKeyCreateAlias != "" {
			body.Alias = pidginhost.PtrString(sshKeyCreateAlias)
		}
		key, _, err := c.AccountAPI.AccountSshKeysCreate(context.Background()).SSHKey(body).Execute()
		if err != nil {
			return fmt.Errorf("creating SSH key: %w", err)
		}
		fmt.Printf("SSH key created (ID: %d, Fingerprint: %s)\n", key.Id, key.Fingerprint)
		return nil
	},
}

var sshKeyDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete an SSH key",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete SSH key %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.AccountAPI.AccountSshKeysDestroy(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("deleting SSH key: %w", err)
		}
		fmt.Printf("SSH key %s deleted.\n", args[0])
		return nil
	},
}

// --- Companies ---

var companyCmd = &cobra.Command{
	Use:   "company",
	Short: "Manage companies",
}

var companyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List companies",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.AccountAPI.AccountCompaniesList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing companies: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME")
			for _, co := range resp.Results {
				output.PrintRow(tw, co.Id, co.Name)
			}
			tw.Flush()
		})
		return nil
	},
}

func pstr[T any](p *T) string {
	if p == nil {
		return "<none>"
	}
	return fmt.Sprintf("%v", *p)
}

func init() {
	sshKeyCreateCmd.Flags().StringVar(&sshKeyCreateAlias, "alias", "", "Key alias/name")
	sshKeyCreateCmd.Flags().StringVar(&sshKeyCreateKey, "key", "", "Public key content (required)")
	sshKeyCreateCmd.MarkFlagRequired("key")

	sshKeyCmd.AddCommand(sshKeyListCmd)
	sshKeyCmd.AddCommand(sshKeyCreateCmd)
	sshKeyCmd.AddCommand(sshKeyDeleteCmd)

	companyCmd.AddCommand(companyListCmd)

	Cmd.AddCommand(profileCmd)
	Cmd.AddCommand(sshKeyCmd)
	Cmd.AddCommand(companyCmd)
}
