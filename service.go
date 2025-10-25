package main

import (
	"context"
	"fmt"

	"github.com/mattn/go-runewidth"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/downloader"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// XiaohongshuService 小红书业务服务
type XiaohongshuService struct{
	browser *headless_browser.Browser // 共享浏览器实例，用于调试时保持打开状态
}

// NewXiaohongshuService 创建小红书服务实例
func NewXiaohongshuService() *XiaohongshuService {
	return &XiaohongshuService{
		browser: browser.NewBrowser(configs.IsHeadless()),
	}
}

// PublishRequest 发布请求
type PublishRequest struct {
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	Images      []string `json:"images" binding:"required,min=1"`
	PublishTime string   `json:"publish_time,omitempty"` // 可选的定时发布时间，格式: "2025-09-12 14:22"（北京时间）
}

// PublishArticleRequest 发布文章请求
type PublishArticleRequest struct {
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	Tags        []string `json:"tags,omitempty"`          // 标签列表
	Images      []string `json:"images" binding:"required,min=1"`
	PublishTime string   `json:"publish_time,omitempty"` // 可选的定时发布时间，格式: "2025-09-12 14:22"（北京时间）
}

// LoginStatusResponse 登录状态响应
type LoginStatusResponse struct {
	IsLoggedIn bool   `json:"is_logged_in"`
	Username   string `json:"username,omitempty"`
}

// PublishResponse 发布响应
type PublishResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Images  int    `json:"images"`
	Status  string `json:"status"`
	PostID  string `json:"post_id,omitempty"`
}

// FeedsListResponse Feeds列表响应
type FeedsListResponse struct {
	Feeds []xiaohongshu.Feed `json:"feeds"`
	Count int                `json:"count"`
}

// CheckLoginStatus 检查登录状态
func (s *XiaohongshuService) CheckLoginStatus(ctx context.Context) (*LoginStatusResponse, error) {
	page := s.browser.NewPage()

	loginAction := xiaohongshu.NewLogin(page)

	isLoggedIn, err := loginAction.CheckLoginStatus(ctx)
	if err != nil {
		return nil, err
	}

	response := &LoginStatusResponse{
		IsLoggedIn: isLoggedIn,
		Username:   configs.Username,
	}

	return response, nil
}

// PublishContent 发布内容
func (s *XiaohongshuService) PublishContent(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	// 验证标题长度
	// 小红书限制：最大40个单位长度
	// 中文/日文/韩文占2个单位，英文/数字占1个单位
	if titleWidth := runewidth.StringWidth(req.Title); titleWidth > 40 {
		return nil, fmt.Errorf("标题长度超过限制")
	}

	// 处理图片：下载URL图片或使用本地路径
	imagePaths, err := s.processImages(req.Images)
	if err != nil {
		return nil, err
	}

	// 构建发布内容
	content := xiaohongshu.PublishImageContent{
		Title:       req.Title,
		Content:     req.Content,
		ImagePaths:  imagePaths,
		PublishTime: req.PublishTime,
	}

	// 执行发布
	if err := s.publishContent(ctx, content); err != nil {
		return nil, err
	}

	response := &PublishResponse{
		Title:   req.Title,
		Content: req.Content,
		Images:  len(imagePaths),
		Status:  "发布完成",
	}

	return response, nil
}

// processImages 处理图片列表，支持URL下载和本地路径
func (s *XiaohongshuService) processImages(images []string) ([]string, error) {
	processor := downloader.NewImageProcessor()
	return processor.ProcessImages(images)
}

// publishContent 执行内容发布
func (s *XiaohongshuService) publishContent(ctx context.Context, content xiaohongshu.PublishImageContent) error {
	page := s.browser.NewPage()

	action, err := xiaohongshu.NewPublishImageAction(page)
	if err != nil {
		return err
	}

	// 执行发布
	return action.Publish(ctx, content)
}

// ListFeeds 获取Feeds列表
func (s *XiaohongshuService) ListFeeds(ctx context.Context) (*FeedsListResponse, error) {
	page := s.browser.NewPage()

	// 创建 Feeds 列表 action
	action := xiaohongshu.NewFeedsListAction(page)

	// 获取 Feeds 列表
	feeds, err := action.GetFeedsList(ctx)
	if err != nil {
		return nil, err
	}

	response := &FeedsListResponse{
		Feeds: feeds,
		Count: len(feeds),
	}

	return response, nil
}

func (s *XiaohongshuService) SearchFeeds(ctx context.Context, keyword string) (*FeedsListResponse, error) {
	page := s.browser.NewPage()

	action := xiaohongshu.NewSearchAction(page)

	feeds, err := action.Search(ctx, keyword)
	if err != nil {
		return nil, err
	}

	response := &FeedsListResponse{
		Feeds: feeds,
		Count: len(feeds),
	}

	return response, nil
}

// GetFeedDetail 获取Feed详情
func (s *XiaohongshuService) GetFeedDetail(ctx context.Context, feedID, xsecToken string) (*FeedDetailResponse, error) {
	page := s.browser.NewPage()

	// 创建 Feed 详情 action
	action := xiaohongshu.NewFeedDetailAction(page)

	// 获取 Feed 详情
	result, err := action.GetFeedDetail(ctx, feedID, xsecToken)
	if err != nil {
		return nil, err
	}

	response := &FeedDetailResponse{
		FeedID: feedID,
		Data:   result,
	}

	return response, nil
}

// PostCommentToFeed 发表评论到Feed
func (s *XiaohongshuService) PostCommentToFeed(ctx context.Context, feedID, xsecToken, content string) (*PostCommentResponse, error) {
	page := s.browser.NewPage()

	// 创建 Feed 评论 action
	action := xiaohongshu.NewCommentFeedAction(page)

	// 发表评论
	err := action.PostComment(ctx, feedID, xsecToken, content)
	if err != nil {
		return nil, err
	}

	response := &PostCommentResponse{
		FeedID:  feedID,
		Success: true,
		Message: "评论发表成功",
	}

	return response, nil
}

// LikeFeed 点赞或取消点赞Feed
func (s *XiaohongshuService) LikeFeed(ctx context.Context, feedID, xsecToken string) (*LikeFeedResponse, error) {
	page := s.browser.NewPage()

	// 创建 Feed 点赞 action
	action := xiaohongshu.NewLikeFeedAction(page)

	// 执行点赞操作
	result, err := action.LikePost(ctx, feedID, xsecToken)
	if err != nil {
		return nil, err
	}

	// 构建响应消息
	var message string
	switch result.Action {
	case "liked":
		message = "点赞成功"
	case "unliked":
		message = "取消点赞成功"
	default:
		message = "点赞状态无变化"
	}

	response := &LikeFeedResponse{
		FeedID:    feedID,
		Success:   true,
		Message:   message,
		Liked:     result.Liked,
		LikeCount: result.LikeCount,
	}

	return response, nil
}

// CollectFeed 收藏或取消收藏Feed
func (s *XiaohongshuService) CollectFeed(ctx context.Context, feedID, xsecToken string) (*CollectFeedResponse, error) {
	page := s.browser.NewPage()

	// 创建 Feed 收藏 action
	action := xiaohongshu.NewCollectFeedAction(page)

	// 执行收藏操作
	result, err := action.CollectPost(ctx, feedID, xsecToken)
	if err != nil {
		return nil, err
	}

	// 构建响应消息
	var message string
	switch result.Action {
	case "collected":
		message = "收藏成功"
	case "uncollected":
		message = "取消收藏成功"
	default:
		message = "收藏状态无变化"
	}

	response := &CollectFeedResponse{
		FeedID:       feedID,
		Success:      true,
		Message:      message,
		Collected:    result.Collected,
		CollectCount: result.CollectCount,
	}

	return response, nil
}

// PublishArticle 发布文章
func (s *XiaohongshuService) PublishArticle(ctx context.Context, req *PublishArticleRequest) (*PublishResponse, error) {
	// 验证标题长度
	if titleWidth := runewidth.StringWidth(req.Title); titleWidth > 64 {
		return nil, fmt.Errorf("文章标题长度超过限制（最大64字符）")
	}

	// 处理图片：下载URL图片或使用本地路径
	imagePaths, err := s.processImages(req.Images)
	if err != nil {
		return nil, err
	}

	// 构建发布内容
	content := xiaohongshu.PublishArticleContent{
		Title:       req.Title,
		Content:     req.Content,
		Tags:        req.Tags,
		ImagePaths:  imagePaths,
		PublishTime: req.PublishTime,
	}

	// 执行发布
	if err := s.publishArticle(ctx, content); err != nil {
		return nil, err
	}

	response := &PublishResponse{
		Title:   req.Title,
		Content: req.Content,
		Images:  len(imagePaths),
		Status:  "文章发布完成",
	}

	return response, nil
}

// publishArticle 执行文章发布
func (s *XiaohongshuService) publishArticle(ctx context.Context, content xiaohongshu.PublishArticleContent) error {
	page := s.browser.NewPage()

	action, err := xiaohongshu.NewPublishArticleAction(page)
	if err != nil {
		return err
	}

	// 执行发布
	return action.Publish(ctx, content)
}

// Close 关闭浏览器实例，用于清理资源
func (s *XiaohongshuService) Close() {
	if s.browser != nil {
		s.browser.Close()
	}
}
