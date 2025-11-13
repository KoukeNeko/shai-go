package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/helpers"
)

const (
	msgNoCachedResponses = "No cached responses."
)

// NewCacheCommand creates the cache command with all subcommands
func NewCacheCommand(container *app.Container) *cobra.Command {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspect or clear response cache",
	}

	cacheCmd.AddCommand(
		newCacheListCommand(container),
		newCacheClearCommand(container),
		newCacheSizeCommand(container),
		newCacheStatsCommand(container),
		newCacheConfigCommand(container),
	)

	return cacheCmd
}

// newCacheListCommand creates the 'cache list' subcommand
func newCacheListCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cache entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listCacheEntries(cmd.OutOrStdout(), container)
		},
	}
}

// newCacheClearCommand creates the 'cache clear' subcommand
func newCacheClearCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear cache directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return clearCache(container)
		},
	}
}

// newCacheSizeCommand creates the 'cache size' subcommand
func newCacheSizeCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "size",
		Short: "Show cache size",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showCacheSize(cmd.OutOrStdout(), container)
		},
	}
}

// newCacheStatsCommand creates the 'cache stats' subcommand
func newCacheStatsCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show cache settings and per-model counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showCacheStats(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
}

// newCacheConfigCommand creates the 'cache config' subcommand
func newCacheConfigCommand(container *app.Container) *cobra.Command {
	var ttl string
	var maxEntries int

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Update cache TTL/max entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateCacheConfiguration(cmd.Context(), container, ttl, maxEntries)
		},
	}

	cmd.Flags().StringVar(&ttl, "ttl", "", "Cache TTL duration (e.g. 30m, 2h)")
	cmd.Flags().IntVar(&maxEntries, "max", 0, "Max cache entries")
	return cmd
}

// listCacheEntries lists all cache entries
func listCacheEntries(out io.Writer, container *app.Container) error {
	if container.CacheStore == nil {
		return fmt.Errorf("cache store unavailable")
	}

	entries, err := container.CacheStore.Entries()
	if err != nil {
		return fmt.Errorf("failed to retrieve cache entries: %w", err)
	}

	for _, entry := range entries {
		fmt.Fprintf(out, "%s | %s | %s\n",
			entry.Key,
			entry.Model,
			entry.CreatedAt.Format(TimestampFormat))
	}

	return nil
}

// clearCache clears the cache directory
func clearCache(container *app.Container) error {
	if container.CacheStore == nil {
		return fmt.Errorf("cache store unavailable")
	}

	if err := container.CacheStore.Clear(); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	return nil
}

// showCacheSize displays the cache directory size
func showCacheSize(out io.Writer, container *app.Container) error {
	if container.CacheStore == nil {
		return fmt.Errorf("cache store unavailable")
	}

	dir := container.CacheStore.Dir()
	totalSize, err := calculateDirectorySize(dir)
	if err != nil {
		return fmt.Errorf("failed to calculate cache size: %w", err)
	}

	fmt.Fprintf(out, "Cache directory: %s\nSize: %d bytes\n", dir, totalSize)
	return nil
}

// showCacheStats displays cache settings and per-model statistics
func showCacheStats(ctx context.Context, out io.Writer, container *app.Container) error {
	cache := container.CacheStore
	if cache == nil {
		return fmt.Errorf("cache store unavailable")
	}

	settings := cache.Settings()
	entries, err := cache.Entries()
	if err != nil {
		return fmt.Errorf("failed to retrieve cache entries: %w", err)
	}

	// Display overall settings
	fmt.Fprintf(out, "Cache TTL: %s\nMax entries: %d\nCurrent entries: %d\n",
		settings.TTL,
		settings.MaxEntries,
		len(entries))

	// Calculate per-model counts
	modelCounts := calculateModelCounts(entries)

	if len(modelCounts) == 0 {
		fmt.Fprintln(out, msgNoCachedResponses)
		return nil
	}

	// Display per-model statistics
	fmt.Fprintln(out, "Entries per model:")
	topModels := helpers.CalculateTopCommands(modelCounts, len(modelCounts))
	for _, stat := range topModels {
		fmt.Fprintf(out, "  %s: %d\n", stat.Command, stat.Count)
	}

	return nil
}

// updateCacheConfiguration updates cache TTL and/or max entries
func updateCacheConfiguration(ctx context.Context, container *app.Container, ttl string, maxEntries int) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	updated := cfg.Cache

	// Validate and update TTL if provided
	if ttl != "" {
		if _, err := time.ParseDuration(ttl); err != nil {
			return fmt.Errorf("invalid ttl: %w", err)
		}
		updated.TTL = ttl
	}

	// Update max entries if provided
	if maxEntries > 0 {
		updated.MaxEntries = maxEntries
	}

	cfg.Cache = updated

	if err := helpers.SaveConfigWithValidation(container, cfg); err != nil {
		return err
	}

	// Update the cache store with new settings
	if container.CacheStore != nil {
		_ = container.CacheStore.Update(updated)
	}

	return nil
}

// calculateDirectorySize calculates the total size of a directory
func calculateDirectorySize(dirPath string) (int64, error) {
	var totalSize int64

	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files that can't be accessed
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil // Skip files that can't be stat'd
		}

		totalSize += info.Size()
		return nil
	})

	if err != nil {
		return 0, err
	}

	return totalSize, nil
}

// calculateModelCounts calculates the number of cache entries per model
func calculateModelCounts(entries []domain.CacheEntry) map[string]int {
	counts := make(map[string]int)

	for _, entry := range entries {
		counts[entry.Model]++
	}

	return counts
}
