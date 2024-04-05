package commands

import (
	"log/slog"
	"os"

	"github.com/everFinance/goar"
	"github.com/liteseed/aogo"
	"github.com/liteseed/edge/internal/contracts"
	"github.com/liteseed/edge/internal/cron"
	"github.com/liteseed/edge/internal/database"
	"github.com/liteseed/edge/internal/server"
	"github.com/liteseed/edge/internal/store"
	"github.com/urfave/cli/v2"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Start = &cli.Command{
	Name:  "start",
	Usage: "Start the bundler node on this system",
	Flags: []cli.Flag{
		&cli.PathFlag{Name: "config", Aliases: []string{"c"}, Value: "./config.json", Usage: "path to config value"},
	},
	Action: start,
}

func start(context *cli.Context) error {
	config := readConfig(context)
	logger := slog.New(slog.NewJSONHandler(&lumberjack.Logger{
		Filename:   config.Log,
		MaxSize:    2, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   //days
		Compress:   true, // disabled by default
	}, nil))

	database, err := database.New(config.Database)
	if err != nil {
		logger.Error(
			"failed: database connect",
			"error", err,
		)
		os.Exit(1)
	}

	wallet, err := goar.NewWalletFromPath(config.Signer, config.Node)
	if err != nil {
		logger.Error(
			"failed: wallet load",
			"error", err,
		)
		os.Exit(1)
	}
	itemSigner, err := goar.NewItemSigner(wallet.Signer)
	if err != nil {
		logger.Error(
			"failed: item-signer create",
			"error", err,
		)
		os.Exit(1)
	}

	store := store.New(config.Store)

	ao, err := aogo.New()
	if err != nil {
		logger.Error(
			"failed to connect to AO",
			"error", err,
		)
		os.Exit(1)
	}

	contracts := contracts.New(ao, itemSigner)

	c, err := cron.New(cron.WthContracts(contracts), cron.WithDatabase(database), cron.WithWallet(wallet), cron.WithStore(store))
	if err != nil {
		logger.Error(
			"failed: cron load",
			"error", err,
		)
		os.Exit(1)
	}

	err = c.PostBundle("* * * * *")
	if err != nil {
		logger.Error(
			"failed to start bundle posting service",
			"error", err,
		)
		os.Exit(1)
	}

	err = c.Notify()
	if err != nil {
		logger.Error(
			"failed to start notification service",
			"error", err,
		)
		os.Exit(1)
	}
	c.Start()

	s := server.New(contracts, database, logger, store)
	s.Run(":8080")

	return nil
}
