package app

import (
	"path"
	"sync/atomic"
	"time"

	"github.com/crazy-max/ftpgrab/v7/internal/config"
	"github.com/crazy-max/ftpgrab/v7/internal/grabber"
	"github.com/crazy-max/ftpgrab/v7/internal/journal"
	"github.com/crazy-max/ftpgrab/v7/internal/notif"
	"github.com/docker/go-units"
	"github.com/hako/durafmt"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// FtpGrab represents an active ftpgrab object
type FtpGrab struct {
	cfg     *config.Config
	cron    *cron.Cron
	notif   *notif.Client
	grabber *grabber.Client
	jobID   cron.EntryID
	locker  uint32
}

// New creates new ftpgrab instance
func New(cfg *config.Config) (*FtpGrab, error) {
	return &FtpGrab{
		cfg: cfg,
		cron: cron.New(cron.WithParser(cron.NewParser(
			cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		)),
	}, nil
}

// Start starts ftpgrab
func (fg *FtpGrab) Start() error {
	var err error

	// Run on startup
	if fg.cfg.Cli.RunOnStart {
		fg.Run()
	}

	// Init scheduler if defined
	if len(fg.cfg.Cli.Schedule) == 0 {
		return nil
	}
	if fg.jobID, err = fg.cron.AddJob(fg.cfg.Cli.Schedule, fg); err != nil {
		return err
	}
	log.Info().Msgf("Cron initialized with schedule %s", fg.cfg.Cli.Schedule)

	// Start scheduler
	fg.cron.Start()
	log.Info().Msgf("Next run in %s (%s)",
		durafmt.Parse(time.Until(fg.cron.Entry(fg.jobID).Next)).LimitFirstN(2).String(),
		fg.cron.Entry(fg.jobID).Next)

	select {}
}

// Run runs ftpgrab process
func (fg *FtpGrab) Run() {
	if !atomic.CompareAndSwapUint32(&fg.locker, 0, 1) {
		log.Warn().Msg("Already running")
		return
	}
	defer atomic.StoreUint32(&fg.locker, 0)
	if fg.jobID > 0 {
		defer log.Info().Msgf("Next run in %s (%s)",
			durafmt.Parse(time.Until(fg.cron.Entry(fg.jobID).Next)).LimitFirstN(2).String(),
			fg.cron.Entry(fg.jobID).Next)
	}

	start := time.Now()
	var err error

	// Notification client
	if fg.notif, err = notif.New(fg.cfg.Notif, fg.cfg.Meta); err != nil {
		log.Fatal().Err(err).Msg("Cannot create notifiers")
	}

	// Grabber client
	if fg.grabber, err = grabber.New(fg.cfg.Download, fg.cfg.Db, nil, fg.cfg.Server); err != nil {
		log.Fatal().Err(err).Msg("Cannot create grabber")
	}
	defer fg.grabber.CloseDB()

	// List files
	files := fg.grabber.ListFiles()
	if len(files) == 0 {
		log.Warn().Msg("No file found from the provided sources")
		return
	}
	log.Info().Strs("sources", fg.grabber.ListExprSrc()).Msgf("%d file(s) found on remote site directories.", len(files))

	log.Info().Msg("Applying filter to file list ...")
	var filteredFiles []grabber.File
	var includedFiles []grabber.File
	var sumIncluded float64 = 0
	var sumDownloading float64 = 0
	var includedSizeUnknown bool
	var downloadingSizeUnknown bool
	formatSize := func(size int64) string {
		if size < 0 {
			return "unknown"
		}
		return units.HumanSize(float64(size))
	}

	for _, file := range files {
		entry := &journal.Entry{
			File:   path.Join(file.SrcDir, file.Info.Name()),
			Status: fg.grabber.GetStatus(file),
		}

		sublogger := log.With().
			Str("src", entry.File).
			Str("dest", file.DestDir).
			Str("size", formatSize(file.Info.Size())).
			Logger()

		if fg.grabber.IsIncluded(file) && !fg.grabber.IsExcluded(file) {
			includedFiles = append(includedFiles, file)
			if file.Info.Size() < 0 {
				includedSizeUnknown = true
			} else {
				sumIncluded += float64(file.Info.Size())
			}
		}

		if entry.Status.IsSkipped() {
			fg.grabber.SkippingFile(entry, sublogger)
		} else {
			filteredFiles = append(filteredFiles, file)
			if file.Info.Size() < 0 {
				downloadingSizeUnknown = true
			} else {
				sumDownloading += float64(file.Info.Size())
			}
		}
	}

	includedSize := units.BytesSize(sumIncluded)
	if includedSizeUnknown {
		includedSize = "unknown"
	}
	downloadingSize := units.BytesSize(sumDownloading)
	if downloadingSizeUnknown {
		downloadingSize = "unknown"
	}
	log.Info().Strs("sources", fg.grabber.ListExprSrc()).Msgf("%d file(s) of %s to be included from remote site.", len(includedFiles), includedSize)
	log.Info().Strs("sources", fg.grabber.ListExprSrc()).Msgf("%d file(s) of %s to be downloaded from remote site.", len(filteredFiles), downloadingSize)

	var jnl journal.Journal
	// Grab
	if fg.cfg.Cli.NoDownload {
		// do not download
		jnl = journal.New().Journal
	} else {
		jnl = fg.grabber.Grab(filteredFiles, fg.cfg.Cli.Concurrency)
	}
	jnl.Duration = time.Since(start)

	log.Info().
		Str("duration", time.Since(start).Round(time.Millisecond).String()).
		Msg("Finished")

	// Check journal before sending report
	if jnl.IsEmpty() {
		log.Warn().Msg("Journal empty, skip sending report")
		return
	}

	// Send notifications
	fg.notif.Send(jnl)
}

// Close closes ftpgrab
func (fg *FtpGrab) Close() {
	fg.grabber.CloseDB()
	if fg.cron != nil {
		fg.cron.Stop()
	}
}
