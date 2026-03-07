package support

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

var (
	outputFormat = cmdutil.OutputFormat
)

var Cmd = &cobra.Command{
	Use:   "support",
	Short: "Manage support tickets",
	Args:  cobra.NoArgs,
}

var TicketCmd = newRootTicketCmd()

// --- Departments ---

var departmentCmd = &cobra.Command{
	Use:   "department",
	Short: "List support departments",
	Args:  cobra.NoArgs,
}

var departmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all support departments",
	Args:  cobra.NoArgs,
	RunE:  runDepartmentList,
}

// --- Tickets ---

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Manage support tickets",
	Args:  cobra.NoArgs,
}

var ticketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tickets",
	Args:  cobra.NoArgs,
	RunE:  runTicketList,
}

var ticketGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get ticket details",
	Args:  cobra.ExactArgs(1),
	RunE:  runTicketGet,
}

var (
	ticketCreateSubject string
	ticketCreateDept    int32
	ticketCreateMessage string
)

var ticketCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a support ticket",
	Args:  cobra.NoArgs,
	RunE:  runTicketCreate,
}

var ticketReplyMessage string

var ticketReplyCmd = &cobra.Command{
	Use:   "reply <id>",
	Short: "Reply to a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runTicketReply,
}

var ticketCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runTicketClose,
}

var ticketReopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runTicketReopen,
}

func runDepartmentList(cmd *cobra.Command, args []string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	depts, _, err := c.SupportAPI.SupportDepartmentsList(context.Background()).Execute()
	if err != nil {
		return fmt.Errorf("listing departments: %w", err)
	}
	format := outputFormat(cmd)
	return output.Print(format, depts, func(w io.Writer) {
		tw := output.NewTabWriter(w)
		output.PrintRow(tw, "ID", "TITLE")
		for _, d := range depts {
			output.PrintRow(tw, d.Id, d.Title)
		}
		tw.Flush()
	})
}

func runTicketList(cmd *cobra.Command, args []string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	tickets, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.TicketList, bool, error) {
		resp, _, err := c.SupportAPI.SupportTicketsList(context.Background()).Page(page).Execute()
		if err != nil {
			return nil, false, err
		}
		return resp.Results, resp.Next.Get() != nil, nil
	})
	if err != nil {
		return fmt.Errorf("listing tickets: %w", err)
	}
	format := outputFormat(cmd)
	return output.Print(format, tickets, func(w io.Writer) {
		tw := output.NewTabWriter(w)
		output.PrintRow(tw, "ID", "SUBJECT", "DEPARTMENT", "STATUS", "CREATED")
		for _, t := range tickets {
			output.PrintRow(tw, t.Id, t.Subject, t.Department, t.Status, t.Created)
		}
		tw.Flush()
	})
}

func runTicketGet(cmd *cobra.Command, args []string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	t, _, err := c.SupportAPI.SupportTicketsRetrieve(context.Background(), args[0]).Execute()
	if err != nil {
		return fmt.Errorf("getting ticket: %w", err)
	}
	format := outputFormat(cmd)
	return output.Print(format, t, func(w io.Writer) {
		tw := output.NewTabWriter(w)
		output.PrintRow(tw, "ID:", t.Id)
		output.PrintRow(tw, "Subject:", t.Subject)
		output.PrintRow(tw, "Department:", t.Department.Title)
		output.PrintRow(tw, "Priority:", t.Priority)
		output.PrintRow(tw, "Status:", t.Status)
		output.PrintRow(tw, "Created:", t.Created)
		output.PrintRow(tw, "Updated:", t.Updated)
		tw.Flush()
		if t.Messages != "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "Messages:")
			fmt.Fprintln(w, t.Messages)
		}
	})
}

func runTicketCreate(cmd *cobra.Command, args []string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	body := *pidginhost.NewTicketCreate(ticketCreateSubject, ticketCreateDept, ticketCreateMessage)
	resp, _, err := c.SupportAPI.SupportTicketsCreate(context.Background()).TicketCreate(body).Execute()
	if err != nil {
		return fmt.Errorf("creating ticket: %w", err)
	}
	fmt.Printf("Ticket created (ID: %d, Subject: %s)\n", resp.Id, resp.Subject)
	return nil
}

func runTicketReply(cmd *cobra.Command, args []string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	body := *pidginhost.NewTicketReply(ticketReplyMessage)
	_, _, err = c.SupportAPI.SupportTicketsReplyCreate(context.Background(), args[0]).TicketReply(body).Execute()
	if err != nil {
		return fmt.Errorf("replying to ticket: %w", err)
	}
	fmt.Printf("Reply sent to ticket %s.\n", args[0])
	return nil
}

func runTicketClose(cmd *cobra.Command, args []string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	_, _, err = c.SupportAPI.SupportTicketsCloseCreate(context.Background(), args[0]).Execute()
	if err != nil {
		return fmt.Errorf("closing ticket: %w", err)
	}
	fmt.Printf("Ticket %s closed.\n", args[0])
	return nil
}

func runTicketReopen(cmd *cobra.Command, args []string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	_, _, err = c.SupportAPI.SupportTicketsReopenCreate(context.Background(), args[0]).Execute()
	if err != nil {
		return fmt.Errorf("reopening ticket: %w", err)
	}
	fmt.Printf("Ticket %s reopened.\n", args[0])
	return nil
}

func newRootTicketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Manage support tickets",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newTicketListAliasCmd())
	cmd.AddCommand(newTicketGetAliasCmd())
	cmd.AddCommand(newTicketCreateAliasCmd())
	cmd.AddCommand(newTicketReplyAliasCmd())
	cmd.AddCommand(newTicketCloseAliasCmd())
	cmd.AddCommand(newTicketReopenAliasCmd())
	return cmd
}

func newTicketListAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tickets",
		Args:  cobra.NoArgs,
		RunE:  runTicketList,
	}
}

func newTicketGetAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get ticket details",
		Args:  cobra.ExactArgs(1),
		RunE:  runTicketGet,
	}
}

func newTicketCreateAliasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a support ticket",
		Args:  cobra.NoArgs,
		RunE:  runTicketCreate,
	}
	addTicketCreateFlags(cmd)
	return cmd
}

func newTicketReplyAliasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reply <id>",
		Short: "Reply to a ticket",
		Args:  cobra.ExactArgs(1),
		RunE:  runTicketReply,
	}
	addTicketReplyFlags(cmd)
	return cmd
}

func newTicketCloseAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "close <id>",
		Short: "Close a ticket",
		Args:  cobra.ExactArgs(1),
		RunE:  runTicketClose,
	}
}

func newTicketReopenAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <id>",
		Short: "Reopen a ticket",
		Args:  cobra.ExactArgs(1),
		RunE:  runTicketReopen,
	}
}

func addTicketCreateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&ticketCreateSubject, "subject", "", "Ticket subject (required)")
	cmd.Flags().Int32Var(&ticketCreateDept, "department", 0, "Department ID (required)")
	cmd.Flags().StringVar(&ticketCreateMessage, "message", "", "Initial message (required)")
	cmd.MarkFlagRequired("subject")
	cmd.MarkFlagRequired("department")
	cmd.MarkFlagRequired("message")
}

func addTicketReplyFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&ticketReplyMessage, "message", "", "Reply message (required)")
	cmd.MarkFlagRequired("message")
}

func init() {
	addTicketCreateFlags(ticketCreateCmd)
	addTicketReplyFlags(ticketReplyCmd)

	departmentCmd.AddCommand(departmentListCmd)

	ticketCmd.AddCommand(ticketListCmd)
	ticketCmd.AddCommand(ticketGetCmd)
	ticketCmd.AddCommand(ticketCreateCmd)
	ticketCmd.AddCommand(ticketReplyCmd)
	ticketCmd.AddCommand(ticketCloseCmd)
	ticketCmd.AddCommand(ticketReopenCmd)

	Cmd.AddCommand(departmentCmd)
	Cmd.AddCommand(ticketCmd)
}
