package ntp

import (
	"context"
	"os"
	"time"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const TimeLayout = "2006-01-02 15:04:05 -0700"

type TimeService interface {
	TimeFunc() func() time.Time
}

type Options struct {
	Context  context.Context
	Server   M.Socksaddr
	Interval time.Duration
	Dialer   N.Dialer
	Logger   logger.Logger
}

var _ TimeService = (*Service)(nil)

type Service struct {
	ctx         context.Context
	cancel      common.ContextCancelCauseFunc
	server      M.Socksaddr
	dialer      N.Dialer
	logger      logger.Logger
	ticker      *time.Ticker
	clockOffset time.Duration
}

func NewService(options Options) *Service {
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := common.ContextWithCancelCause(ctx)
	destination := options.Server
	if !destination.IsValid() {
		destination = M.Socksaddr{
			Fqdn: "time.google.com",
		}
	}
	if destination.Port == 0 {
		destination.Port = 123
	}
	var interval time.Duration
	if options.Interval > 0 {
		interval = options.Interval
	} else {
		interval = 30 * time.Minute
	}
	var dialer N.Dialer
	if options.Dialer != nil {
		dialer = options.Dialer
	} else {
		dialer = N.SystemDialer
	}
	return &Service{
		ctx:    ctx,
		cancel: cancel,
		server: destination,
		dialer: dialer,
		logger: options.Logger,
		ticker: time.NewTicker(interval),
	}
}

func (s *Service) Start() error {
	err := s.update()
	if err != nil {
		return E.Cause(err, "initialize time")
	}
	s.logger.Info("updated time: ", s.TimeFunc()().Local().Format(TimeLayout))
	go s.loopUpdate()
	return nil
}

func (s *Service) Close() error {
	s.ticker.Stop()
	s.cancel(os.ErrClosed)
	return nil
}

func (s *Service) TimeFunc() func() time.Time {
	return func() time.Time {
		return time.Now().Add(s.clockOffset)
	}
}

func (s *Service) loopUpdate() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
		}
		err := s.update()
		if err == nil {
			s.logger.Debug("updated time: ", s.TimeFunc()().Local().Format(TimeLayout))
		} else {
			s.logger.Warn("update time: ", err)
		}
	}
}

func (s *Service) update() error {
	response, err := Exchange(s.ctx, s.dialer, s.server)
	if err != nil {
		return err
	}
	s.clockOffset = response.ClockOffset
	return nil
}
