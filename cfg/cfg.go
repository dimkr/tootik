/*
Copyright 2023 - 2025 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cfg defines the tootik configuration file format and defaults.
package cfg

import (
	"math"
	"regexp"
	"time"
)

// Config represents a tootik configuration file.
type Config struct {
	DatabaseOptions string

	RequireRegistration             bool
	RequireInvitation               bool
	MaxInvitationsPerUser           *int
	RegistrationInterval            time.Duration
	CertificateApprovalTimeout      time.Duration
	UserNameRegex                   string
	CompiledUserNameRegex           *regexp.Regexp `json:"-"`
	ForbiddenUserNameRegex          string
	CompiledForbiddenUserNameRegex  *regexp.Regexp `json:"-"`
	EnablePortableActorRegistration bool

	MaxPostsLength     int
	MaxPostsPerDay     int64
	PostThrottleFactor int64
	PostThrottleUnit   time.Duration

	EditThrottleFactor float64
	EditThrottleUnit   time.Duration

	ShareThrottleFactor int64
	ShareThrottleUnit   time.Duration

	PollMaxOptions int
	PollDuration   time.Duration

	MaxDisplayNameLength int
	MaxBioLength         int
	MaxMetadataFields    int
	MaxAvatarSize        int64
	MaxAvatarWidth       int
	MaxAvatarHeight      int
	AvatarWidth          int
	AvatarHeight         int
	MinActorEditInterval time.Duration

	MaxFollowsPerUser int

	MaxBookmarksPerUser int
	MinBookmarkInterval time.Duration

	PostsPerPage     int
	PostContextDepth int
	RepliesPerPage   int
	MaxOffset        int

	SharesPerPost int
	QuotesPerPost int

	MaxRequestBodySize int64
	MaxRequestAge      time.Duration

	MaxResponseBodySize int64

	CompactViewMaxRunes int
	CompactViewMaxLines int

	CacheUpdateTimeout time.Duration

	GeminiRequestTimeout time.Duration

	GopherRequestTimeout time.Duration
	LineWidth            int

	GuppyRequestTimeout time.Duration
	MaxGuppySessions    int
	GuppyChunkTimeout   time.Duration
	MaxSentGuppyChunks  int

	DeliveryBatchSize     int
	DeliveryRetryInterval int64
	MaxDeliveryAttempts   int
	DeliveryTimeout       time.Duration
	DeliveryWorkers       int
	DeliveryWorkerBuffer  int

	OutboxPollingInterval time.Duration

	MaxActivitiesQueueSize    int
	ActivitiesBatchSize       int
	ActivitiesPollingInterval time.Duration
	ActivitiesBatchDelay      time.Duration
	ActivityProcessingTimeout time.Duration
	MaxForwardingDepth        int

	MaxRecipients int
	MinActorAge   time.Duration

	ResolverCacheTTL        time.Duration
	ResolverRetryInterval   time.Duration
	ResolverMaxIdleConns    int
	ResolverIdleConnTimeout time.Duration
	MaxInstanceRecoveryTime time.Duration
	MaxResolverRequests     int

	FollowersSyncBatchSize int
	FollowersSyncInterval  time.Duration

	FeedUpdateInterval time.Duration

	NotesTTL          time.Duration
	InvisiblePostsTTL time.Duration
	DeliveryTTL       time.Duration
	ActorTTL          time.Duration
	FeedTTL           time.Duration

	FillNodeInfoUsage bool

	RFC9421Threshold float32
	Ed25519Threshold float32

	DisableIntegrityProofs bool
	MaxGateways            int
}

var defaultMaxInvitationsPerUser = 5

// FillDefaults replaces missing or invalid settings with defaults.
func (c *Config) FillDefaults() {
	if c.DatabaseOptions == "" {
		c.DatabaseOptions = "_journal_mode=WAL&_synchronous=1&_busy_timeout=5000"
	}

	if c.MaxInvitationsPerUser == nil {
		c.MaxInvitationsPerUser = &defaultMaxInvitationsPerUser
	}

	if c.RegistrationInterval <= 0 {
		c.RegistrationInterval = time.Hour
	}

	if c.CertificateApprovalTimeout <= 0 {
		c.CertificateApprovalTimeout = time.Hour * 48
	}

	if c.UserNameRegex == "" {
		c.UserNameRegex = `^[a-zA-Z0-9-_]{4,32}$`
	}

	c.CompiledUserNameRegex = regexp.MustCompile(c.UserNameRegex)

	if c.ForbiddenUserNameRegex == "" {
		c.ForbiddenUserNameRegex = `^(root|localhost|ip6-.*|.*(admin|tootik).*)$`
	}

	c.CompiledForbiddenUserNameRegex = regexp.MustCompile(c.ForbiddenUserNameRegex)

	if c.MaxPostsLength <= 0 {
		c.MaxPostsLength = 500
	}

	if c.MaxPostsPerDay <= 0 {
		c.MaxPostsPerDay = 30
	}

	if c.PostThrottleFactor <= 0 {
		c.PostThrottleFactor = 2
	}

	if c.PostThrottleUnit <= 0 {
		c.PostThrottleUnit = time.Minute
	}

	if c.EditThrottleFactor <= 0 {
		c.EditThrottleFactor = 4
	}

	if c.EditThrottleUnit <= 0 {
		c.EditThrottleUnit = time.Minute
	}

	if c.ShareThrottleFactor <= 0 {
		c.ShareThrottleFactor = 4
	}

	if c.ShareThrottleUnit <= 0 {
		c.ShareThrottleUnit = time.Minute
	}

	if c.PollMaxOptions < 2 {
		c.PollMaxOptions = 5
	}

	if c.PollDuration <= 0 {
		c.PollDuration = time.Hour * 24 * 30
	}

	if c.MaxDisplayNameLength <= 0 {
		c.MaxDisplayNameLength = 30
	}

	if c.MaxBioLength <= 0 {
		c.MaxBioLength = 500
	}

	if c.MaxMetadataFields <= 0 {
		c.MaxMetadataFields = 4
	}

	if c.MaxAvatarSize <= 0 {
		c.MaxAvatarSize = 2 * 1024 * 1024
	}

	if c.MaxAvatarWidth <= 0 {
		c.MaxAvatarWidth = 1024
	}

	if c.MaxAvatarHeight <= 0 {
		c.MaxAvatarHeight = 1024
	}

	if c.AvatarWidth <= 0 {
		c.AvatarWidth = 400
	}

	if c.AvatarHeight <= 0 {
		c.AvatarHeight = 400
	}

	if c.MinActorEditInterval <= 0 {
		c.MinActorEditInterval = time.Minute * 30
	}

	if c.MaxFollowsPerUser <= 0 {
		c.MaxFollowsPerUser = 150
	}

	if c.MaxBookmarksPerUser <= 0 {
		c.MaxBookmarksPerUser = 100
	}

	if c.MinBookmarkInterval <= 0 {
		c.MinBookmarkInterval = time.Second * 5
	}

	if c.PostsPerPage <= 0 {
		c.PostsPerPage = 30
	}

	if c.PostContextDepth <= 0 {
		c.PostContextDepth = 5
	}

	if c.RepliesPerPage <= 0 {
		c.RepliesPerPage = 10
	}

	if c.MaxOffset <= 0 {
		c.MaxOffset = c.PostsPerPage * 30
	}

	if c.SharesPerPost <= 0 {
		c.SharesPerPost = 10
	}

	if c.QuotesPerPost <= 0 {
		c.QuotesPerPost = 10
	}

	if c.MaxRequestBodySize <= 0 {
		c.MaxRequestBodySize = 1024 * 1024
	}

	if c.MaxRequestAge <= 0 {
		c.MaxRequestAge = time.Minute * 5
	}

	if c.MaxResponseBodySize <= 0 {
		c.MaxResponseBodySize = 1024 * 1024
	}

	if c.CompactViewMaxRunes <= 0 {
		c.CompactViewMaxRunes = 200
	}
	if c.CompactViewMaxLines <= 0 {
		c.CompactViewMaxLines = 4
	}

	if c.CacheUpdateTimeout <= 0 {
		c.CacheUpdateTimeout = time.Second * 5
	}

	if c.GeminiRequestTimeout <= 0 {
		c.GeminiRequestTimeout = time.Second * 30
	}

	if c.GopherRequestTimeout <= 0 {
		c.GopherRequestTimeout = time.Second * 30
	}

	if c.LineWidth <= 0 {
		c.LineWidth = 70
	}

	if c.GuppyRequestTimeout <= 0 {
		c.GuppyRequestTimeout = time.Second * 30
	}

	if c.MaxGuppySessions <= 0 {
		c.MaxGuppySessions = 30
	}

	if c.GuppyChunkTimeout <= 0 {
		c.GuppyChunkTimeout = time.Second * 2
	}

	if c.MaxSentGuppyChunks <= 0 {
		c.MaxSentGuppyChunks = 8
	}

	if c.DeliveryBatchSize <= 0 {
		c.DeliveryBatchSize = 16
	}

	if c.DeliveryRetryInterval <= 0 {
		c.DeliveryRetryInterval = int64((time.Hour / 2) / time.Second)
	}

	if c.MaxDeliveryAttempts <= 0 {
		c.MaxDeliveryAttempts = 5
	}

	if c.DeliveryTimeout <= 0 {
		c.DeliveryTimeout = time.Minute * 5
	}

	if c.DeliveryWorkers <= 0 || c.DeliveryWorkers > math.MaxInt {
		c.DeliveryWorkers = 4
	}

	if c.DeliveryWorkerBuffer <= 0 {
		c.DeliveryWorkerBuffer = 16
	}

	if c.OutboxPollingInterval <= 0 {
		c.OutboxPollingInterval = time.Second * 5
	}

	if c.MaxActivitiesQueueSize <= 0 {
		c.MaxActivitiesQueueSize = 10000
	}

	if c.ActivitiesBatchSize <= 0 {
		c.ActivitiesBatchSize = 64
	}

	if c.ActivitiesPollingInterval <= 0 {
		c.ActivitiesPollingInterval = time.Second * 5
	}

	if c.ActivitiesBatchDelay <= 0 {
		c.ActivitiesBatchDelay = time.Millisecond * 100
	}

	if c.ActivityProcessingTimeout <= 0 {
		c.ActivityProcessingTimeout = time.Second * 15
	}

	if c.MaxForwardingDepth <= 0 {
		c.MaxForwardingDepth = 5
	}

	if c.MaxRecipients <= 0 {
		c.MaxRecipients = 10
	}

	if c.MinActorAge <= 0 {
		c.MinActorAge = time.Hour * 24
	}

	if c.ResolverCacheTTL <= 0 {
		c.ResolverCacheTTL = time.Hour * 24 * 3
	}

	if c.ResolverRetryInterval <= 0 {
		c.ResolverRetryInterval = time.Hour * 6
	}

	if c.ResolverMaxIdleConns <= 0 {
		c.ResolverMaxIdleConns = 128
	}

	if c.ResolverIdleConnTimeout <= 0 {
		c.ResolverIdleConnTimeout = time.Minute
	}

	if c.MaxInstanceRecoveryTime <= 0 {
		c.MaxInstanceRecoveryTime = time.Hour * 24 * 30
	}

	if c.MaxResolverRequests <= 0 {
		c.MaxResolverRequests = 16
	}

	if c.FollowersSyncBatchSize <= 0 {
		c.FollowersSyncBatchSize = 64
	}

	if c.FollowersSyncInterval <= 0 {
		c.FollowersSyncInterval = time.Hour * 24 * 3
	}

	if c.FeedUpdateInterval <= 0 {
		c.FeedUpdateInterval = time.Minute * 10
	}

	if c.NotesTTL <= 0 {
		c.NotesTTL = time.Hour * 24 * 30
	}

	if c.InvisiblePostsTTL <= 0 {
		c.InvisiblePostsTTL = time.Hour * 24 * 14
	}

	if c.DeliveryTTL <= 0 {
		c.DeliveryTTL = time.Hour * 24 * 7
	}

	if c.ActorTTL <= 0 {
		c.ActorTTL = time.Hour * 24 * 7
	}

	if c.FeedTTL <= 0 {
		c.FeedTTL = time.Hour * 24 * 7
	}

	if c.RFC9421Threshold <= 0 || c.RFC9421Threshold > 1 {
		c.RFC9421Threshold = 0.95
	}

	if c.Ed25519Threshold <= 0 || c.Ed25519Threshold > 1 {
		c.Ed25519Threshold = 0.98
	}

	if c.MaxGateways <= 0 {
		c.MaxGateways = 10
	}
}
