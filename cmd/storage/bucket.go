package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var bucketCmd = &cobra.Command{
	Use:     "bucket",
	Aliases: []string{"buckets"},
	Short:   "Manage S3 buckets",
	Args:    cobra.NoArgs,
}

// The list route is not paginated: it answers a bare array, like volumes.
var bucketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all buckets",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudBucketsList(cmd.Context()).Execute()
		if err != nil {
			return cmdutil.APIError("listing buckets", err)
		}
		if resp == nil {
			return fmt.Errorf("listing buckets: server returned no bucket list")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "FULL NAME", "QUOTA GB", "USED", "OBJECTS", "PUBLIC", "STATUS", "REGION")
			for _, b := range resp {
				output.PrintRow(tw, b.Id, b.Name, b.FullName, b.QuotaGb,
					b.UsedBytes, b.ObjectCount, b.PublicRead, b.Status, b.Region)
			}
			tw.Flush()
		})
	},
}

var bucketGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get bucket details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		b, _, err := c.CloudAPI.CloudBucketsRetrieve(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("getting bucket", err)
		}
		if b == nil {
			return fmt.Errorf("getting bucket %d: server returned no bucket", id)
		}
		if b.Id != id {
			return fmt.Errorf("getting bucket %d: server returned bucket %d", id, b.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), b, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", b.Id)
			output.PrintRow(tw, "Name:", b.Name)
			output.PrintRow(tw, "Full name:", b.FullName)
			output.PrintRow(tw, "Quota (GB):", b.QuotaGb)
			output.PrintRow(tw, "Used bytes:", b.UsedBytes)
			output.PrintRow(tw, "Objects:", b.ObjectCount)
			output.PrintRow(tw, "Public read:", b.PublicRead)
			output.PrintRow(tw, "Status:", b.Status)
			output.PrintRow(tw, "Endpoint:", b.Endpoint)
			output.PrintRow(tw, "Region:", b.Region)
			output.PrintRow(tw, "Created:", b.Created)
			tw.Flush()
		})
	},
}

var (
	bucketCreateName   string
	bucketCreateQuota  int32
	bucketCreatePublic bool
)

var bucketCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a bucket",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// A bucket is billed on usage, so creating one costs money.
		prompt := fmt.Sprintf("Create bucket %q with a %d GB quota? This is a paid resource.",
			bucketCreateName, bucketCreateQuota)
		if bucketCreatePublic {
			prompt += " Uploaded objects will be readable by anyone on the internet."
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			prompt) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewBucketCreate(bucketCreateName, bucketCreateQuota)
		body.SetPublicRead(bucketCreatePublic)
		b, _, err := c.CloudAPI.CloudBucketsCreate(cmd.Context()).BucketCreate(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating bucket", err)
		}
		if b == nil {
			return fmt.Errorf("creating bucket: server returned no bucket")
		}
		if b.Name != bucketCreateName || b.QuotaGb != bucketCreateQuota || b.PublicRead != bucketCreatePublic {
			return fmt.Errorf("creating bucket: requested name=%q quota_gb=%d public_read=%t, server returned name=%q quota_gb=%d public_read=%t",
				bucketCreateName, bucketCreateQuota, bucketCreatePublic, b.Name, b.QuotaGb, b.PublicRead)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), b, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", b.Id)
			output.PrintRow(tw, "Name:", b.Name)
			output.PrintRow(tw, "Full name:", b.FullName)
			output.PrintRow(tw, "Quota (GB):", b.QuotaGb)
			output.PrintRow(tw, "Public read:", b.PublicRead)
			output.PrintRow(tw, "Status:", b.Status)
			output.PrintRow(tw, "Endpoint:", b.Endpoint)
			tw.Flush()
		})
	},
}

var bucketDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a bucket and everything in it",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Delete bucket %d and every object in it? This cannot be undone.", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		// Deletion is asynchronous: the route answers 202 with the status it
		// moved to, not 204, so report what it actually said.
		resp, _, err := c.CloudAPI.CloudBucketsDestroy(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("deleting bucket", err)
		}
		if resp == nil {
			return fmt.Errorf("deleting bucket %d: server returned no cancellation result", id)
		}
		if resp.Id != id {
			return fmt.Errorf("deleting bucket %d: server reported cancellation for bucket %d", id, resp.Id)
		}
		if resp.Status != "cancelling" {
			return fmt.Errorf("deleting bucket %d: server reported status %q instead of cancelling", id, resp.Status)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Bucket %d %s.\n", resp.Id, resp.Status)
	},
}

var bucketResizeQuota int32

var bucketResizeCmd = &cobra.Command{
	Use:   "resize <id>",
	Short: "Change a bucket's quota",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		// Quota drives the bill.
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Resize bucket %d to %d GB? This changes what it costs.", id, bucketResizeQuota)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewBucketResize(bucketResizeQuota)
		b, _, err := c.CloudAPI.CloudBucketsResizeCreate(cmd.Context(), id).BucketResize(body).Execute()
		if err != nil {
			return cmdutil.APIError("resizing bucket", err)
		}
		if b == nil {
			return fmt.Errorf("resizing bucket %d: server returned no bucket", id)
		}
		if b.Id != id {
			return fmt.Errorf("resizing bucket %d: server returned bucket %d", id, b.Id)
		}
		// A 200 still carrying a different quota means nothing changed. Asking
		// for the quota it already has is a no-op, not an error: scripts
		// converge on a desired state and should not have to check first.
		if b.QuotaGb != bucketResizeQuota {
			return fmt.Errorf("resizing bucket %d: asked for %d GB, bucket still reports %d GB",
				id, bucketResizeQuota, b.QuotaGb)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), b,
			"Bucket %d resized to %d GB.\n", b.Id, b.QuotaGb)
	},
}

var (
	bucketVisibilityPublic  bool
	bucketVisibilityPrivate bool
)

var bucketVisibilityCmd = &cobra.Command{
	Use:   "visibility <id>",
	Short: "Make a bucket's objects world-readable, or private again",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if bucketVisibilityPublic == bucketVisibilityPrivate {
			return fmt.Errorf("pass exactly one of --public or --private")
		}
		public := bucketVisibilityPublic
		// Going public exposes every object to the internet; going private
		// only takes access away, so it needs no confirmation.
		if public && !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Make every object in bucket %d readable by anyone on the internet?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewBucketVisibility(public)
		b, _, err := c.CloudAPI.CloudBucketsVisibilityCreate(cmd.Context(), id).BucketVisibility(body).Execute()
		if err != nil {
			return cmdutil.APIError("changing bucket visibility", err)
		}
		if b == nil {
			return fmt.Errorf("changing bucket %d visibility: server returned no bucket", id)
		}
		if b.Id != id {
			return fmt.Errorf("changing bucket %d visibility: server returned bucket %d", id, b.Id)
		}
		// Saying "now public" when the bucket is still private is worse than
		// an error: the operator stops checking.
		if b.PublicRead != public {
			return fmt.Errorf("changing bucket %d visibility: asked for public_read=%t, bucket still reports %t",
				id, public, b.PublicRead)
		}
		state := "private"
		if b.PublicRead {
			state = "public-read"
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), b,
			"Bucket %d is now %s.\n", b.Id, state)
	},
}

var bucketCredentialsCmd = &cobra.Command{
	Use:     "credentials",
	Aliases: []string{"creds"},
	Short:   "Reveal or rotate a bucket's S3 credentials",
	Args:    cobra.NoArgs,
}

var errCredentialOutput = errors.New("credential output failed")

func validateCredentials(creds *pidginhost.BucketCredentials) error {
	// After rotation the new pair is the irreplaceable part of the response.
	// Empty display metadata must not suppress keys that the operator can still
	// use and may not be able to retrieve from this one-time response again.
	if creds == nil || creds.AccessKey == "" || creds.SecretKey == "" {
		return errors.New("server returned incomplete bucket credentials")
	}
	return nil
}

// printCredentials renders in memory before touching the destination. If the
// writer fails, its error is not propagated because a writer that echoes its
// payload in the error would copy the secret to stderr when Cobra reports it.
func printCredentials(cmd *cobra.Command, creds *pidginhost.BucketCredentials) error {
	var rendered bytes.Buffer
	if err := output.Print(&rendered, cmdutil.OutputFormat(cmd), creds, func(w io.Writer) {
		tw := output.NewTabWriter(w)
		output.PrintRow(tw, "Bucket:", creds.Bucket)
		output.PrintRow(tw, "Endpoint:", creds.Endpoint)
		output.PrintRow(tw, "Region:", creds.Region)
		output.PrintRow(tw, "Access key:", creds.AccessKey)
		output.PrintRow(tw, "Secret key:", creds.SecretKey)
		_ = tw.Flush()
	}); err != nil {
		return errCredentialOutput
	}
	payload := rendered.Bytes()
	n, err := cmd.OutOrStdout().Write(payload)
	if err != nil || n != len(payload) {
		return errCredentialOutput
	}
	return nil
}

func rotationFailure(id int32, cause error) error {
	return fmt.Errorf("%w; the previous pair may already be invalid; run %q to retrieve the current pair; do not retry rotation",
		cause, fmt.Sprintf("phctl storage bucket credentials reveal %d", id))
}

var bucketCredentialsRevealCmd = &cobra.Command{
	Use:   "reveal <id>",
	Short: "Print a bucket's access key and secret",
	Long: "Print a bucket's access key and secret.\n\n" +
		"These grant full read/write access to the bucket. The command asks before\n" +
		"printing them; pass -f to skip the prompt in scripts. The reveal is\n" +
		"recorded in the account's audit log and is rate-limited server-side.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		// Secrets do not appear on someone's terminal by accident.
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Print bucket %d's access key and secret to this terminal?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		creds, _, err := c.CloudAPI.CloudBucketsCredentialsRevealCreate(cmd.Context(), id).Execute()
		if err != nil {
			// The response body is the credentials; it must not reach an error.
			return cmdutil.APIErrorRedacted("revealing bucket credentials", err)
		}
		if err := validateCredentials(creds); err != nil {
			return fmt.Errorf("revealing bucket credentials: %w", err)
		}
		if err := printCredentials(cmd, creds); err != nil {
			return fmt.Errorf("revealing bucket credentials: %w", err)
		}
		return nil
	},
}

var bucketCredentialsRotateCmd = &cobra.Command{
	Use:   "rotate <id>",
	Short: "Issue a new access key and secret, invalidating the old pair",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Rotate bucket %d's credentials? The current key and secret stop working immediately.", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		creds, _, err := c.CloudAPI.CloudBucketsCredentialsRotateCreate(cmd.Context(), id).Execute()
		if err != nil {
			// The keys are already rotated by the time this can fail, so the
			// message must not be the place the new secret shows up.
			return rotationFailure(id, cmdutil.APIErrorRedacted("rotating bucket credentials", err))
		}
		if err := validateCredentials(creds); err != nil {
			return rotationFailure(id, fmt.Errorf("rotating bucket credentials: %w", err))
		}
		if err := printCredentials(cmd, creds); err != nil {
			return rotationFailure(id, fmt.Errorf("rotating bucket credentials: %w", err))
		}
		return nil
	},
}

func init() {
	bucketCreateCmd.Flags().StringVar(&bucketCreateName, "name", "", "Bucket name (required)")
	bucketCreateCmd.Flags().Int32Var(&bucketCreateQuota, "quota", 0, "Quota in GB (required)")
	bucketCreateCmd.Flags().BoolVar(&bucketCreatePublic, "public", false, "Make objects world-readable")
	_ = bucketCreateCmd.MarkFlagRequired("name")
	_ = bucketCreateCmd.MarkFlagRequired("quota")

	bucketResizeCmd.Flags().Int32Var(&bucketResizeQuota, "quota", 0, "New quota in GB (required)")
	_ = bucketResizeCmd.MarkFlagRequired("quota")

	bucketVisibilityCmd.Flags().BoolVar(&bucketVisibilityPublic, "public", false, "Make objects world-readable")
	bucketVisibilityCmd.Flags().BoolVar(&bucketVisibilityPrivate, "private", false, "Make objects private")
	bucketVisibilityCmd.MarkFlagsMutuallyExclusive("public", "private")

	bucketCredentialsCmd.AddCommand(bucketCredentialsRevealCmd)
	bucketCredentialsCmd.AddCommand(bucketCredentialsRotateCmd)

	bucketCmd.AddCommand(bucketListCmd)
	bucketCmd.AddCommand(bucketGetCmd)
	bucketCmd.AddCommand(bucketCreateCmd)
	bucketCmd.AddCommand(bucketDeleteCmd)
	bucketCmd.AddCommand(bucketResizeCmd)
	bucketCmd.AddCommand(bucketVisibilityCmd)
	bucketCmd.AddCommand(bucketCredentialsCmd)
}
