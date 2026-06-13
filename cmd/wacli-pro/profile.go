package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/spf13/cobra"
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
			data, err := runLiveOrDelegate(flags, "profile.set-about", map[string]any{"text": text},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opProfileSetAbout(ctx, a, text)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
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
			data, err := runLiveOrDelegate(flags, "profile.set-photo", map[string]any{"file": filePath},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opProfileSetPhoto(ctx, a, filePath)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
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
			data, err := runLiveOrDelegate(flags, "profile.remove-photo", map[string]any{},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opProfileRemovePhoto(ctx, a)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	return cmd
}
