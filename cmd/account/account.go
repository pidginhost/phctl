package account

import (
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var Cmd = &cobra.Command{
	Use:   "account",
	Short: "Manage account, profile, SSH keys, and companies",
	Args:  cobra.NoArgs,
}

// --- Profile ---

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "View account profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		var profile client.RawProfile
		if err := client.RawGet(cmd.Context(), "/api/account/profile", &profile); err != nil {
			return fmt.Errorf("getting profile: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, profile, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "First Name:", profile.FirstName)
			output.PrintRow(tw, "Last Name:", profile.LastName)
			output.PrintRow(tw, "Funds:", profile.Funds)
			output.PrintRow(tw, "Phone:", profile.Phone)
			tw.Flush()
		})
	},
}

// --- SSH Keys ---

var sshKeyCmd = &cobra.Command{
	Use:     "ssh-key",
	Aliases: []string{"ssh"},
	Short:   "Manage SSH keys",
	Args:    cobra.NoArgs,
}

var sshKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SSH keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		keys, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.SSHKey, bool, error) {
			resp, _, err := c.AccountAPI.AccountSshKeysList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing SSH keys: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, keys, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ALIAS", "FINGERPRINT")
			for _, k := range keys {
				output.PrintRow(tw, k.Id, output.Pstr(k.Alias), k.Fingerprint)
			}
			tw.Flush()
		})
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
		key, _, err := c.AccountAPI.AccountSshKeysCreate(cmd.Context()).SSHKey(body).Execute()
		if err != nil {
			return fmt.Errorf("creating SSH key: %w", err)
		}
		cmd.Printf("SSH key created (ID: %d, Fingerprint: %s)\n", key.Id, key.Fingerprint)
		return nil
	},
}

var sshKeyDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete an SSH key",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete SSH key %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.AccountAPI.AccountSshKeysDestroy(cmd.Context(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("deleting SSH key: %w", err)
		}
		cmd.Printf("SSH key %s deleted.\n", args[0])
		return nil
	},
}

// --- Companies ---

var companyCmd = &cobra.Command{
	Use:   "company",
	Short: "Manage companies",
	Args:  cobra.NoArgs,
}

var companyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List companies",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		companies, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.Company, bool, error) {
			resp, _, err := c.AccountAPI.AccountCompaniesList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing companies: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, companies, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME")
			for _, co := range companies {
				output.PrintRow(tw, co.Id, co.Name)
			}
			tw.Flush()
		})
	},
}

// --- API Tokens ---

var apiTokenCmd = &cobra.Command{
	Use:   "api-token",
	Short: "Manage API tokens",
	Args:  cobra.NoArgs,
}

var apiTokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		tokens, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.APITokenList, bool, error) {
			resp, _, err := c.AccountAPI.AccountApiTokensList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing API tokens: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, tokens, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "CREATED")
			for _, t := range tokens {
				output.PrintRow(tw, t.Id, t.Name, t.Created)
			}
			tw.Flush()
		})
	},
}

var apiTokenCreateName string

var apiTokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewAPITokenCreate(0, apiTokenCreateName, "", "")
		resp, _, err := c.AccountAPI.AccountApiTokensCreate(cmd.Context()).APITokenCreate(body).Execute()
		if err != nil {
			return fmt.Errorf("creating API token: %w", err)
		}
		cmd.Printf("API token created (Name: %s)\n", resp.Name)
		cmd.Printf("Token: %s\n", resp.Key)
		cmd.Println("Save this token — it will not be shown again.")
		return nil
	},
}

var apiTokenDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete an API token",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete API token %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.AccountAPI.AccountApiTokensDestroy(cmd.Context(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("deleting API token: %w", err)
		}
		cmd.Printf("API token %s deleted.\n", args[0])
		return nil
	},
}

// --- Email History ---

var emailCmd = &cobra.Command{
	Use:   "email",
	Short: "View account email history",
	Args:  cobra.NoArgs,
}

var emailListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sent emails",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		emails, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.EmailHistory, bool, error) {
			resp, _, err := c.AccountAPI.AccountEmailsList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing emails: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, emails, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "SUBJECT", "ADDRESS", "DATE", "READ")
			for _, e := range emails {
				output.PrintRow(tw, e.Id, e.Subject, e.Address, e.Date, e.Read)
			}
			tw.Flush()
		})
	},
}

func init() {
	sshKeyCreateCmd.Flags().StringVar(&sshKeyCreateAlias, "alias", "", "Key alias/name")
	sshKeyCreateCmd.Flags().StringVar(&sshKeyCreateKey, "key", "", "Public key content (required)")
	sshKeyCreateCmd.MarkFlagRequired("key")

	sshKeyCmd.AddCommand(sshKeyListCmd)
	sshKeyCmd.AddCommand(sshKeyCreateCmd)
	sshKeyCmd.AddCommand(sshKeyDeleteCmd)

	apiTokenCreateCmd.Flags().StringVar(&apiTokenCreateName, "name", "", "Token name (required)")
	apiTokenCreateCmd.MarkFlagRequired("name")

	apiTokenCmd.AddCommand(apiTokenListCmd)
	apiTokenCmd.AddCommand(apiTokenCreateCmd)
	apiTokenCmd.AddCommand(apiTokenDeleteCmd)

	emailCmd.AddCommand(emailListCmd)

	companyCmd.AddCommand(companyListCmd)

	Cmd.AddCommand(profileCmd)
	Cmd.AddCommand(sshKeyCmd)
	Cmd.AddCommand(companyCmd)
	Cmd.AddCommand(apiTokenCmd)
	Cmd.AddCommand(emailCmd)
}
