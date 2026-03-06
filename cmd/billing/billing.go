package billing

import (
	"context"
	"fmt"
	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"
	"io"

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
	Use:     "billing",
	Aliases: []string{"bill"},
	Short:   "Manage billing, invoices, funds, and services",
}

// --- Funds / Balance ---

var fundsCmd = &cobra.Command{
	Use:   "funds",
	Short: "Manage account funds and balance",
}

var fundsBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Show current account balance",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.BillingAPI.BillingFundsList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("getting balance: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			for _, f := range resp {
				output.PrintRow(tw, "Balance:", f.Balance)
				output.PrintRow(tw, "Threshold Type:", f.ThresholdType)
			}
			tw.Flush()
		})
	},
}

var fundsLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show funds activity log",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		logs, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.FundsLog, bool, error) {
			resp, _, err := c.BillingAPI.BillingFundsLogList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing funds log: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, logs, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "OPERATION", "AMOUNT", "BALANCE", "DATE")
			for _, l := range logs {
				output.PrintRow(tw, l.Id, l.Operation, l.Amount, l.Balance, l.Date.Format("2006-01-02 15:04"))
			}
			tw.Flush()
		})
	},
}

// --- Deposits ---

var depositCmd = &cobra.Command{
	Use:   "deposit",
	Short: "Manage deposits",
}

var depositListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all deposits",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		deposits, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.Deposit, bool, error) {
			resp, _, err := c.BillingAPI.BillingDepositsList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing deposits: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, deposits, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "AMOUNT", "TOTAL", "STATUS", "DATE")
			for _, d := range deposits {
				output.PrintRow(tw, d.Id, d.Amount, d.Total, d.Status, d.Created.Format("2006-01-02"))
			}
			tw.Flush()
		})
	},
}

var depositGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get deposit details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		d, _, err := c.BillingAPI.BillingDepositsRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting deposit: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, d, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", d.Id)
			output.PrintRow(tw, "Amount:", d.Amount)
			output.PrintRow(tw, "VAT:", d.VatValue)
			output.PrintRow(tw, "Total:", d.Total)
			output.PrintRow(tw, "Status:", d.Status)
			output.PrintRow(tw, "Created:", d.Created.Format("2006-01-02"))
			tw.Flush()
		})
	},
}

var depositCreateAmount int32

var depositCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a deposit",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDepositCreate(depositCreateAmount)
		resp, _, err := c.BillingAPI.BillingDepositsCreate(context.Background()).DepositCreate(body).Execute()
		if err != nil {
			return fmt.Errorf("creating deposit: %w", err)
		}
		fmt.Printf("Deposit created (ID: %d, Amount: %.2f)\n", resp.Id, resp.Amount)
		return nil
	},
}

// --- Invoices ---

var invoiceCmd = &cobra.Command{
	Use:   "invoice",
	Short: "Manage invoices",
}

var invoiceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all invoices",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		invoices, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.InvoiceList, bool, error) {
			resp, _, err := c.BillingAPI.BillingInvoicesList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing invoices: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, invoices, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "PROFORMA", "FISCAL", "TOTAL", "STATUS", "DATE")
			for _, inv := range invoices {
				output.PrintRow(tw, inv.Id, inv.NumberProforma, inv.NumberFiscal, inv.Total, inv.Status, inv.InvoiceDate)
			}
			tw.Flush()
		})
	},
}

var invoiceGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get invoice details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		inv, _, err := c.BillingAPI.BillingInvoicesRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting invoice: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, inv, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", inv.Id)
			output.PrintRow(tw, "Proforma:", inv.NumberProforma)
			output.PrintRow(tw, "Fiscal:", inv.NumberFiscal)
			output.PrintRow(tw, "Subtotal:", inv.Subtotal)
			output.PrintRow(tw, "VAT:", inv.VatValue)
			output.PrintRow(tw, "Total:", inv.Total)
			output.PrintRow(tw, "Status:", inv.Status)
			output.PrintRow(tw, "Date:", inv.InvoiceDate)
			output.PrintRow(tw, "Payment:", inv.PaymentMethod)
			tw.Flush()
		})
	},
}

var invoicePayCmd = &cobra.Command{
	Use:   "pay <id>",
	Short: "Pay an invoice using account funds",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Pay invoice %s using account funds?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.BillingAPI.BillingInvoicesPayWithFundsCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("paying invoice: %w", err)
		}
		fmt.Printf("Invoice paid: %s\n", resp.Message)
		return nil
	},
}

// --- Services ---

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage billing services",
}

var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		services, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.ServiceList, bool, error) {
			resp, _, err := c.BillingAPI.BillingServicesList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing services: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, services, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "STATUS", "PRICE", "CYCLE", "NEXT INVOICE")
			for _, s := range services {
				output.PrintRow(tw, s.Id, s.Hostname, s.Status, s.Price, s.BillingCycle, s.NextInvoice)
			}
			tw.Flush()
		})
	},
}

var serviceGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get service details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		s, _, err := c.BillingAPI.BillingServicesRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting service: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, s, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", s.Id)
			output.PrintRow(tw, "Hostname:", s.Hostname)
			output.PrintRow(tw, "Status:", s.Status)
			output.PrintRow(tw, "Price:", s.Price)
			output.PrintRow(tw, "Billing Cycle:", s.BillingCycle)
			output.PrintRow(tw, "Next Invoice:", s.NextInvoice)
			output.PrintRow(tw, "Auto Pay:", s.AutoPayment)
			output.PrintRow(tw, "Company:", s.Company)
			tw.Flush()
		})
	},
}

var serviceCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Cancel service %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.BillingAPI.BillingServicesCancelCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("cancelling service: %w", err)
		}
		fmt.Printf("Service cancelled: %s\n", resp.Message)
		return nil
	},
}

var serviceAutoPayCmd = &cobra.Command{
	Use:   "toggle-auto-pay <id>",
	Short: "Toggle auto-payment for a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.BillingAPI.BillingServicesToggleAutoPaymentCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("toggling auto-pay: %w", err)
		}
		state := "enabled"
		if !resp.AutoPayment {
			state = "disabled"
		}
		fmt.Printf("Auto-pay %s: %s\n", state, resp.Message)
		return nil
	},
}

// --- Subscriptions ---

var subscriptionCmd = &cobra.Command{
	Use:   "subscription",
	Short: "Manage subscriptions",
}

var subscriptionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all subscriptions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		subs, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.Subscription, bool, error) {
			resp, _, err := c.BillingAPI.BillingSubscriptionsList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing subscriptions: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, subs, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "STATUS", "TOTAL", "CREATED")
			for _, s := range subs {
				output.PrintRow(tw, s.Id, s.ServiceHostname, s.Status, s.Total, s.CreationDate.Format("2006-01-02"))
			}
			tw.Flush()
		})
	},
}

var subscriptionGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get subscription details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		s, _, err := c.BillingAPI.BillingSubscriptionsRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting subscription: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, s, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", s.Id)
			output.PrintRow(tw, "Hostname:", s.ServiceHostname)
			output.PrintRow(tw, "Status:", s.Status)
			output.PrintRow(tw, "Subtotal:", s.Subtotal)
			output.PrintRow(tw, "VAT:", s.VatValue)
			output.PrintRow(tw, "Total:", s.Total)
			output.PrintRow(tw, "Created:", s.CreationDate.Format("2006-01-02"))
			tw.Flush()
		})
	},
}

func init() {
	depositCreateCmd.Flags().Int32Var(&depositCreateAmount, "amount", 0, "Deposit amount in EUR (required)")
	depositCreateCmd.MarkFlagRequired("amount")

	fundsCmd.AddCommand(fundsBalanceCmd)
	fundsCmd.AddCommand(fundsLogCmd)

	depositCmd.AddCommand(depositListCmd)
	depositCmd.AddCommand(depositGetCmd)
	depositCmd.AddCommand(depositCreateCmd)

	invoiceCmd.AddCommand(invoiceListCmd)
	invoiceCmd.AddCommand(invoiceGetCmd)
	invoiceCmd.AddCommand(invoicePayCmd)

	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceGetCmd)
	serviceCmd.AddCommand(serviceCancelCmd)
	serviceCmd.AddCommand(serviceAutoPayCmd)

	subscriptionCmd.AddCommand(subscriptionListCmd)
	subscriptionCmd.AddCommand(subscriptionGetCmd)

	Cmd.AddCommand(fundsCmd)
	Cmd.AddCommand(depositCmd)
	Cmd.AddCommand(invoiceCmd)
	Cmd.AddCommand(serviceCmd)
	Cmd.AddCommand(subscriptionCmd)
}
