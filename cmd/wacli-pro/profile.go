package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/UAEpro/wacli-pro/internal/out"
)

func newProfileCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage your WhatsApp profile",
	}
	cmd.AddCommand(newProfileSetAboutCmd(flags))
	cmd.AddCommand(newProfileSetPhotoCmd(flags))
	cmd.AddCommand(newProfileRemovePhotoCmd(flags))
	return cmd
}

func newProfileSetAboutCmd(flags *rootFlags) *cobra.Command {
	var text string
	cmd := &cobra.Command{
		Use:   "set-about",
		Short: "Set your profile 'About' text",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("text") {
				return fmt.Errorf("--text is required")
			}
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}
			if err := a.Connect(ctx, false, nil); err != nil {
				return err
			}

			if err := a.WA().SetStatusMessage(ctx, text); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"about": text})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "about text (use empty to clear)")
	return cmd
}

func newProfileSetPhotoCmd(flags *rootFlags) *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "set-photo",
		Short: "Set your profile photo (JPEG)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(filePath) == "" {
				return fmt.Errorf("--file is required")
			}
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}
			if err := a.Connect(ctx, false, nil); err != nil {
				return err
			}

			avatar, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read photo file: %w", err)
			}

			pictureID, err := a.WA().SetProfilePhoto(ctx, avatar)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"picture_id": pictureID})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "path to JPEG photo file")
	return cmd
}

func newProfileRemovePhotoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-photo",
		Short: "Remove your profile photo",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}
			if err := a.Connect(ctx, false, nil); err != nil {
				return err
			}

			if _, err := a.WA().SetProfilePhoto(ctx, nil); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"removed": true})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	return cmd
}
