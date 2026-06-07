package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/UAEpro/wacli-pro/internal/store"
	"github.com/UAEpro/wacli-pro/internal/wa"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type WAClient interface {
	Close()
	IsAuthed() bool
	IsConnected() bool
	Connect(ctx context.Context, opts wa.ConnectOptions) error

	AddEventHandler(handler func(interface{})) uint32
	RemoveEventHandler(id uint32)
	ReconnectWithBackoff(ctx context.Context, minDelay, maxDelay time.Duration) error

	ResolveChatName(ctx context.Context, chat types.JID, pushName string) string
	ResolveLIDToPN(ctx context.Context, jid types.JID) types.JID
	GetContact(ctx context.Context, jid types.JID) (types.ContactInfo, error)
	GetAllContacts(ctx context.Context) (map[types.JID]types.ContactInfo, error)

	GetJoinedGroups(ctx context.Context) ([]*types.GroupInfo, error)
	GetGroupInfo(ctx context.Context, jid types.JID) (*types.GroupInfo, error)
	SetGroupName(ctx context.Context, jid types.JID, name string) error
	UpdateGroupParticipants(ctx context.Context, group types.JID, users []types.JID, action wa.GroupParticipantAction) ([]types.GroupParticipant, error)
	SetGroupTopic(ctx context.Context, jid types.JID, topic string) error
	SetGroupPhoto(ctx context.Context, jid types.JID, avatar []byte) (string, error)
	SetGroupLocked(ctx context.Context, jid types.JID, locked bool) error
	SetGroupAnnounce(ctx context.Context, jid types.JID, announce bool) error
	SetGroupJoinApprovalMode(ctx context.Context, jid types.JID, mode bool) error
	SetGroupMemberAddMode(ctx context.Context, jid types.JID, mode types.GroupMemberAddMode) error
	GetGroupInviteLink(ctx context.Context, group types.JID, reset bool) (string, error)
	JoinGroupWithLink(ctx context.Context, code string) (types.JID, error)
	CreateGroup(ctx context.Context, name string, participants []types.JID) (*types.GroupInfo, error)
	GetGroupRequestParticipants(ctx context.Context, jid types.JID) ([]types.GroupParticipantRequest, error)
	UpdateGroupRequestParticipants(ctx context.Context, jid types.JID, users []types.JID, approve bool) ([]types.GroupParticipant, error)
	LeaveGroup(ctx context.Context, group types.JID) error

	// Newsletters/Channels
	GetSubscribedNewsletters(ctx context.Context) ([]*types.NewsletterMetadata, error)
	GetNewsletterInfo(ctx context.Context, jid types.JID) (*types.NewsletterMetadata, error)
	GetNewsletterInfoWithInvite(ctx context.Context, key string) (*types.NewsletterMetadata, error)
	FollowNewsletter(ctx context.Context, jid types.JID) error
	UnfollowNewsletter(ctx context.Context, jid types.JID) error
	NewsletterToggleMute(ctx context.Context, jid types.JID, mute bool) error

	// Profile
	SetStatusMessage(ctx context.Context, msg string) error
	SetProfilePhoto(ctx context.Context, avatar []byte) (string, error)

	// Polls
	BuildPollCreation(name string, options []string, maxSelections int) *waProto.Message

	SendText(ctx context.Context, to types.JID, text string) (types.MessageID, error)
	SendTextReply(ctx context.Context, to types.JID, text string, replyToID string, participantJID *types.JID, quoted *waProto.Message) (types.MessageID, error)
	SendProtoMessage(ctx context.Context, to types.JID, msg *waProto.Message) (types.MessageID, error)
	RevokeMessage(ctx context.Context, chat types.JID, msgID types.MessageID) error
	EditMessage(ctx context.Context, chat types.JID, msgID types.MessageID, newText string) (types.MessageID, error)
	Upload(ctx context.Context, data []byte, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error)
	DownloadMediaToFile(ctx context.Context, directPath string, encFileHash, fileHash, mediaKey []byte, fileLength uint64, mediaType, mmsType string, targetPath string) (int64, error)

	SendReaction(ctx context.Context, chat types.JID, targetMsgID types.MessageID, emoji string) error
	SendLocation(ctx context.Context, to types.JID, lat, lng float64, name, address string) (types.MessageID, error)

	SendPresence(ctx context.Context, state types.Presence) error
	SendChatPresence(ctx context.Context, jid types.JID, state types.ChatPresence, media types.ChatPresenceMedia) error
	DecryptReaction(ctx context.Context, reaction *events.Message) (*waProto.ReactionMessage, error)
	RequestHistorySyncOnDemand(ctx context.Context, lastKnown types.MessageInfo, count int) (types.MessageID, error)
	Logout(ctx context.Context) error

	ArchiveChat(ctx context.Context, target types.JID, archive bool, lastMsgTS time.Time, lastMsgKey *waCommon.MessageKey) error
	PinChat(ctx context.Context, target types.JID, pin bool) error
	MuteChat(ctx context.Context, target types.JID, mute bool, duration time.Duration) error
	MarkChatAsRead(ctx context.Context, target types.JID, read bool, lastMsgTS time.Time, lastMsgKey *waCommon.MessageKey) error
}

type Options struct {
	StoreDir      string
	Version       string
	JSON          bool
	Events        *out.EventWriter
	AllowUnauthed bool
}

type App struct {
	opts   Options
	waOnce sync.Once
	waErr  error
	events *out.EventWriter
	wa     WAClient
	db     *store.DB
}

func New(opts Options) (*App, error) {
	if opts.StoreDir == "" {
		return nil, fmt.Errorf("store dir is required")
	}
	if err := os.MkdirAll(opts.StoreDir, 0700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	indexPath := filepath.Join(opts.StoreDir, "wacli.db")

	db, err := store.Open(indexPath)
	if err != nil {
		return nil, err
	}

	ev := opts.Events
	if ev == nil {
		ev = out.NewEventWriter(os.Stderr, false)
	}

	return &App{opts: opts, events: ev, db: db}, nil
}

func (a *App) OpenWA() error {
	a.waOnce.Do(func() {
		if a.wa != nil {
			return
		}
		sessionPath := filepath.Join(a.opts.StoreDir, "session.db")
		cli, err := wa.New(wa.Options{
			StorePath: sessionPath,
		})
		if err != nil {
			a.waErr = err
			return
		}
		a.wa = cli
	})
	return a.waErr
}

func (a *App) Close() {
	if a.wa != nil {
		a.wa.Close()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
}

func (a *App) EnsureAuthed() error {
	if err := a.OpenWA(); err != nil {
		return err
	}
	if a.wa.IsAuthed() {
		return nil
	}
	return fmt.Errorf("not authenticated; run `wacli auth`")
}

func (a *App) WA() WAClient             { return a.wa }
func (a *App) DB() *store.DB            { return a.db }
func (a *App) Events() *out.EventWriter { return a.events }
func (a *App) StoreDir() string         { return a.opts.StoreDir }
func (a *App) Version() string          { return a.opts.Version }
func (a *App) AllowUnauthed() bool      { return a.opts.AllowUnauthed }

func (a *App) Connect(ctx context.Context, allowQR bool, qrWriter func(string)) error {
	if err := a.OpenWA(); err != nil {
		return err
	}
	return a.wa.Connect(ctx, wa.ConnectOptions{
		AllowQR:  allowQR,
		OnQRCode: qrWriter,
	})
}
