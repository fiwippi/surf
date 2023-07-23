package surf

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
)

func Run(token, spotifyID, spotifySecret string) error {
	// Create the client
	c, err := newClient(token, spotifyID, spotifySecret)
	if err != nil {
		return err
	}

	// Context for running the bot
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	// Run the bot
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
