package surf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"surf/internal/log"
	"surf/pkg/lava"
)

func Run(token string, conf lava.Config, lavalinkPath string) error {
	// Run Lavalink
	lavalinkCmd := exec.Command("java", "-Djdk.tls.client.protocols=TLSv1.1,TLSv1.2", "-Xmx4G", "-jar", lavalinkPath)
	log.Info().Str("host", conf.Host).Str("port", conf.Port).Msg("connecting to lavalink...")
	if err := lavalinkCmd.Start(); err != nil {
		return fmt.Errorf("failed to start lavalink command: %s", err)
	}

	// Create the client
	c, err := newClient(token, conf)
	if err != nil {
		return err
	}

	// Run the bot
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	if err := c.state.Open(ctx); err != nil {
		return err
	}
	log.Info().Msg("bot loading...")

	// Delete old commands which are no longer required and create new ones
	existingCommands, err := c.state.Commands(c.self.ID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not get existing commands")
	}
	c.deleteOldCommands(existingCommands)
	c.createNewCommands(existingCommands)

	log.Info().Str("user", c.self.Name).Msg("bot ready")
	<-ctx.Done() // block until Ctrl+C
	log.Info().Msg("closing bot...")

	if err := c.state.Close(); err != nil {
		return err
	}
	log.Info().Msg("bot closed")
	return nil
}
