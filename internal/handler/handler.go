// Package handler implements the mixi2 event handler for the OpenSkill rating bot.
package handler

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	modelv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/model/v1"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
	"github.com/yourusername/openskill-rating-bot/internal/config"
	"github.com/yourusername/openskill-rating-bot/internal/db"
	"github.com/yourusername/openskill-rating-bot/internal/rating"
)

// BotID is the mixi2 user ID of the bot itself (@openskill_rating)
const BotID = "openskill_rating"

// matchPattern matches:
// "@openskill_rating @winner が @loser に勝ちました！"
// or "@openskill_rating @winner が @loser に勝ちました"
// The bot mention may be at the start, and user IDs follow @
var matchPattern = regexp.MustCompile(
	`@openskill_rating\s+@(\S+)\s*が\s*@(\S+)\s*に勝ちました[！!]?`,
)

// rankingPattern matches "@openskill_rating ランキング"
var rankingPattern = regexp.MustCompile(`@openskill_rating\s+ランキング`)

// ratingPattern matches "@openskill_rating @username レーティング"
var ratingPattern = regexp.MustCompile(`@openskill_rating\s+@(\S+)\s*レーティング`)

// helpPattern matches "@openskill_rating ヘルプ" or "@openskill_rating help"
var helpPattern = regexp.MustCompile(`@openskill_rating\s+(ヘルプ|help)`)

// Handler processes mixi2 events for the rating bot.
type Handler struct {
	cfg           *config.Config
	store         *db.Store
	apiClient     application_apiv1.ApplicationServiceClient
	authenticator auth.Authenticator
}

// New creates a new Handler.
func New(cfg *config.Config, store *db.Store, authenticator auth.Authenticator) (*Handler, error) {
	// Connect to API server
	apiConn, err := grpc.NewClient(
		cfg.APIAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to API server: %w", err)
	}

	client := application_apiv1.NewApplicationServiceClient(apiConn)

	return &Handler{
		cfg:           cfg,
		store:         store,
		apiClient:     client,
		authenticator: authenticator,
	}, nil
}

// Handle processes incoming mixi2 events.
func (h *Handler) Handle(ctx context.Context, ev *modelv1.Event) error {
	// Use the Body oneof to dispatch event types
	if ev.GetPostCreatedEvent() != nil {
		return h.handlePostCreated(ctx, ev.GetPostCreatedEvent())
	}
	// Ignore other event types
	return nil
}

// handlePostCreated processes post creation events (mentions).
func (h *Handler) handlePostCreated(ctx context.Context, event *modelv1.PostCreatedEvent) error {
	if event.Post == nil {
		return nil
	}

	postText := event.Post.Text
	postID := event.Post.PostId
	issuerID := ""
	if event.Issuer != nil {
		issuerID = event.Issuer.GetUserId()
	}

	log.Printf("Received post from %s: %s", issuerID, postText)

	// Check for match result recording command
	// Format: @openskill_rating @winner が @loser に勝ちました！
	if matches := matchPattern.FindStringSubmatch(postText); len(matches) == 3 {
		return h.handleMatchResult(ctx, postID, issuerID, matches[1], matches[2])
	}

	// Check for ranking command
	if rankingPattern.MatchString(postText) {
		return h.handleRankingRequest(ctx, postID)
	}

	// Check for individual rating query
	if matches := ratingPattern.FindStringSubmatch(postText); len(matches) == 2 {
		return h.handleRatingRequest(ctx, postID, matches[1])
	}

	// Check for help command
	if helpPattern.MatchString(postText) {
		return h.handleHelp(ctx, postID)
	}

	return nil
}

// handleMatchResult processes a match result and updates ratings.
func (h *Handler) handleMatchResult(ctx context.Context, replyToPostID, issuerID, winnerID, loserID string) error {
	// Only the admin can record match results
	if issuerID != h.cfg.AdminUserID {
		log.Printf("Unauthorized match recording attempt by %s (admin: %s)", issuerID, h.cfg.AdminUserID)
		return h.replyToPost(ctx, replyToPostID,
			fmt.Sprintf("⚠️ 勝敗の記録は管理者のみが行えます。"),
		)
	}

	// Prevent self-matches
	if winnerID == loserID {
		return h.replyToPost(ctx, replyToPostID,
			"⚠️ 勝者と敗者が同じユーザーです。",
		)
	}

	// Get current ratings
	winnerRecord, err := h.store.GetPlayer(winnerID)
	if err != nil {
		return fmt.Errorf("failed to get winner rating: %w", err)
	}
	loserRecord, err := h.store.GetPlayer(loserID)
	if err != nil {
		return fmt.Errorf("failed to get loser rating: %w", err)
	}

	winnerBefore := rating.NewRatingWith(winnerRecord.Mu, winnerRecord.Sigma)
	loserBefore := rating.NewRatingWith(loserRecord.Mu, loserRecord.Sigma)

	// Calculate new ratings
	winnerAfter, loserAfter := rating.PlackettLuceRate(winnerBefore, loserBefore)

	// Record match and update ratings in database
	if err := h.store.RecordMatch(winnerID, loserID, winnerBefore, winnerAfter, loserBefore, loserAfter); err != nil {
		return fmt.Errorf("failed to record match: %w", err)
	}

	// Format rating changes
	winnerDelta := winnerAfter.Mu - winnerBefore.Mu
	loserDelta := loserAfter.Mu - loserBefore.Mu

	winnerSign := "+"
	if winnerDelta < 0 {
		winnerSign = ""
	}
	loserSign := ""
	if loserDelta > 0 {
		loserSign = "+"
	}

	// Get updated records for win/loss counts
	winnerUpdated, _ := h.store.GetPlayer(winnerID)
	loserUpdated, _ := h.store.GetPlayer(loserID)

	msg := fmt.Sprintf(
		"🏆 対戦結果を記録しました！\n\n"+
			"✅ @%s が @%s に勝ちました！\n\n"+
			"📊 レーティング更新:\n"+
			"@%s: %.0f → %.0f (%s%.1f) [%dW/%dL]\n"+
			"@%s: %.0f → %.0f (%s%.1f) [%dW/%dL]",
		winnerID, loserID,
		winnerID,
		winnerBefore.Mu, winnerAfter.Mu,
		winnerSign, winnerDelta,
		winnerUpdated.Wins, winnerUpdated.Losses,
		loserID,
		loserBefore.Mu, loserAfter.Mu,
		loserSign, loserDelta,
		loserUpdated.Wins, loserUpdated.Losses,
	)

	log.Printf("Match recorded: %s beat %s (%.1f→%.1f, %.1f→%.1f)",
		winnerID, loserID, winnerBefore.Mu, winnerAfter.Mu, loserBefore.Mu, loserAfter.Mu)

	return h.replyToPost(ctx, replyToPostID, msg)
}

// handleRankingRequest returns the top players ranking.
func (h *Handler) handleRankingRequest(ctx context.Context, replyToPostID string) error {
	players, err := h.store.GetRanking(10)
	if err != nil {
		return fmt.Errorf("failed to get ranking: %w", err)
	}

	if len(players) == 0 {
		return h.replyToPost(ctx, replyToPostID,
			"📊 まだ対戦記録がありません。",
		)
	}

	var sb strings.Builder
	sb.WriteString("🏆 OpenSkill レーティング ランキング\n\n")

	medals := []string{"🥇", "🥈", "🥉"}
	for i, p := range players {
		r := rating.NewRatingWith(p.Mu, p.Sigma)
		var prefix string
		if i < len(medals) {
			prefix = medals[i]
		} else {
			prefix = fmt.Sprintf("%d.", i+1)
		}
		sb.WriteString(fmt.Sprintf("%s @%s: %.0f pt [%dW/%dL]\n",
			prefix, p.UserID, r.Mu, p.Wins, p.Losses))
	}

	return h.replyToPost(ctx, replyToPostID, sb.String())
}

// handleRatingRequest returns a specific player's rating.
func (h *Handler) handleRatingRequest(ctx context.Context, replyToPostID, userID string) error {
	// Remove @ prefix if present
	userID = strings.TrimPrefix(userID, "@")

	record, err := h.store.GetPlayer(userID)
	if err != nil {
		return fmt.Errorf("failed to get player rating: %w", err)
	}

	r := rating.NewRatingWith(record.Mu, record.Sigma)
	winRate := 0.0
	total := record.Wins + record.Losses
	if total > 0 {
		winRate = float64(record.Wins) / float64(total) * 100
	}

	msg := fmt.Sprintf(
		"📊 @%s のレーティング\n\n"+
			"🔢 Rating (μ): %.1f\n"+
			"📉 不確実性 (σ): %.1f\n"+
			"📈 保守的評価: %.1f\n"+
			"🏆 戦績: %dW / %dL (勝率 %.1f%%)\n"+
			"🕐 更新: %s",
		userID,
		r.Mu,
		r.Sigma,
		r.Ordinal(),
		record.Wins, record.Losses, winRate,
		record.UpdatedAt.Format("2006/01/02 15:04"),
	)

	return h.replyToPost(ctx, replyToPostID, msg)
}

// handleHelp returns usage instructions.
func (h *Handler) handleHelp(ctx context.Context, replyToPostID string) error {
	msg := `📖 OpenSkill Rating Bot 使い方

【勝敗記録】(管理者のみ)
@openskill_rating @勝者ID が @敗者ID に勝ちました！

【ランキング確認】
@openskill_rating ランキング

【個人レーティング確認】
@openskill_rating @ユーザーID レーティング

【このヘルプ】
@openskill_rating ヘルプ

レーティングシステム: OpenSkill (Plackett-Luce)
初期レーティング: 1000`

	return h.replyToPost(ctx, replyToPostID, msg)
}

// replyToPost sends a reply to a post with rate limiting awareness.
func (h *Handler) replyToPost(ctx context.Context, replyToPostID, text string) error {
	// Get authorized context
	authCtx, err := h.authenticator.AuthorizedContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authorized context: %w", err)
	}

	// Truncate if too long (mixi2 limit is 149 chars)
	if len([]rune(text)) > 149 {
		runes := []rune(text)
		text = string(runes[:146]) + "..."
	}

	postReplyCtx, cancel := context.WithTimeout(authCtx, 10*time.Second)
	defer cancel()

	_, err = h.apiClient.CreatePost(postReplyCtx, &application_apiv1.CreatePostRequest{
		Text:            text,
		InReplyToPostId: &replyToPostID,
		// Note: PublishingType defaults to timeline delivery
	})
	if err != nil {
		return fmt.Errorf("failed to create reply post: %w", err)
	}

	log.Printf("Reply sent to post %s", replyToPostID)
	return nil
}
