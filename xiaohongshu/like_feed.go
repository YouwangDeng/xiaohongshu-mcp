package xiaohongshu

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// LikeFeedAction 表示 Feed 点赞动作
type LikeFeedAction struct {
	page *rod.Page
}

// NewLikeFeedAction 创建 Feed 点赞动作
func NewLikeFeedAction(page *rod.Page) *LikeFeedAction {
	return &LikeFeedAction{page: page}
}

// LikePost 点赞或取消点赞 Feed
func (l *LikeFeedAction) LikePost(ctx context.Context, feedID, xsecToken string) (*LikeResult, error) {
	page := l.page.Context(ctx).Timeout(60 * time.Second)

	// 构建详情页 URL
	url := makeFeedDetailURL(feedID, xsecToken)

	logrus.Infof("Opening feed detail page for like action: %s", url)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()

	time.Sleep(2 * time.Second)

	// 尝试多种可能的点赞按钮选择器
	likeButton, err := l.findLikeButton(page)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find like button")
	}

	// 获取当前点赞状态
	currentLiked, currentCount, err := l.getCurrentLikeStatus(page)
	if err != nil {
		logrus.Warnf("Failed to get current like status: %v", err)
		// 继续执行，不因为获取状态失败而中断
	}

	logrus.Infof("Current like status - Liked: %v, Count: %s", currentLiked, currentCount)

	// 点击点赞按钮
	if err := likeButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, errors.Wrap(err, "failed to click like button")
	}

	time.Sleep(2 * time.Second)

	// 获取更新后的点赞状态
	newLiked, newCount, err := l.getCurrentLikeStatus(page)
	if err != nil {
		logrus.Warnf("Failed to get updated like status: %v", err)
		// 如果无法获取新状态，假设操作成功并切换状态
		newLiked = !currentLiked
		newCount = currentCount
	}

	logrus.Infof("Updated like status - Liked: %v, Count: %s", newLiked, newCount)

	return &LikeResult{
		Liked:     newLiked,
		LikeCount: newCount,
		Action:    l.getActionType(currentLiked, newLiked),
	}, nil
}

// LikeResult 点赞操作结果
type LikeResult struct {
	Liked     bool   `json:"liked"`
	LikeCount string `json:"like_count"`
	Action    string `json:"action"` // "liked" or "unliked"
}

// findLikeButton 查找点赞按钮
func (l *LikeFeedAction) findLikeButton(page *rod.Page) (*rod.Element, error) {
	// 设置超时时间，避免无限等待
	timeout := time.After(30 * time.Second)

	// 尝试多种可能的点赞按钮选择器
	selectors := []string{
		// 小红书特定的点赞按钮选择器
		"button[class*='like-lottie']",
		"button[class*='like']",
		"div[class*='like-lottie']",
		"div[class*='like']",
		"span[class*='like-lottie']",
		"span[class*='like']",
		// 基于交互区域的查找
		".interact-info button[class*='like']",
		".interact-info div[class*='like']",
		".note-interact button[class*='like']",
		".note-interact div[class*='like']",
		// 基于图标的查找
		"button svg[class*='heart']",
		"div svg[class*='heart']",
		"span svg[class*='heart']",
		// 基于文本的查找
		"button:contains('点赞')",
		"div:contains('点赞')",
		"span:contains('点赞')",
		// 通用选择器
		".like-btn",
		".like-button",
		"[data-testid*='like']",
		// 基于位置的查找
		".interact-info button:first-child",
		".interact-info div:first-child",
		".note-interact button:first-child",
		".note-interact div:first-child",
	}

	for _, selector := range selectors {
		select {
		case <-timeout:
			return nil, errors.New("timeout while searching for like button")
		default:
			if element, err := page.Element(selector); err == nil {
				logrus.Infof("Found like button with selector: %s", selector)
				return element, nil
			}
		}
	}

	// 如果所有选择器都失败，尝试调试页面结构
	logrus.Warn("All like button selectors failed, attempting to debug page structure")
	l.debugPageStructure(page)

	return nil, errors.New("like button not found with any known selector")
}

// getCurrentLikeStatus 获取当前点赞状态
func (l *LikeFeedAction) getCurrentLikeStatus(page *rod.Page) (bool, string, error) {
	// 尝试从页面中获取点赞状态
	// 这里可能需要根据实际的页面结构来调整

	// 方法1: 从 __INITIAL_STATE__ 获取
	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ && window.__INITIAL_STATE__.note) {
			const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
			for (const key in noteDetailMap) {
				const note = noteDetailMap[key].note;
				if (note && note.interactInfo) {
					return JSON.stringify({
						liked: note.interactInfo.liked,
						count: note.interactInfo.likedCount
					});
				}
			}
		}
		return null;
	}`).String()

	if result != "null" && result != "" {
		// 尝试解析JSON结果
		var data struct {
			Liked bool   `json:"liked"`
			Count string `json:"count"`
		}
		if err := json.Unmarshal([]byte(result), &data); err == nil {
			return data.Liked, data.Count, nil
		}
	}

	// 方法2: 从DOM元素获取
	// 查找点赞按钮的状态
	likeSelectors := []string{
		"button[class*='like'][class*='active']",
		"div[class*='like'][class*='active']",
		"span[class*='like'][class*='active']",
		".like-btn.active",
		".like-button.active",
	}

	for _, selector := range likeSelectors {
		if _, err := page.Element(selector); err == nil {
			// 找到激活状态的点赞按钮，说明已点赞
			// 尝试获取点赞数
			count := l.extractLikeCount(page)
			return true, count, nil
		}
	}

	// 默认返回未点赞状态
	count := l.extractLikeCount(page)
	return false, count, nil
}

// extractLikeCount 从页面中提取点赞数
func (l *LikeFeedAction) extractLikeCount(page *rod.Page) string {
	// 尝试多种方式获取点赞数
	countSelectors := []string{
		".like-count",
		".like-count span",
		"[class*='like'][class*='count']",
		".interact-info .like-count",
		".note-interact .like-count",
	}

	for _, selector := range countSelectors {
		if element, err := page.Element(selector); err == nil {
			if text, err := element.Text(); err == nil && text != "" {
				return text
			}
		}
	}

	return "0"
}

// getActionType 获取操作类型
func (l *LikeFeedAction) getActionType(oldLiked, newLiked bool) string {
	if !oldLiked && newLiked {
		return "liked"
	} else if oldLiked && !newLiked {
		return "unliked"
	}
	return "no_change"
}

// debugPageStructure 调试页面结构，帮助找到正确的选择器
func (l *LikeFeedAction) debugPageStructure(page *rod.Page) {
	logrus.Info("=== Debugging page structure for like button ===")

	// 查找所有可能的交互元素
	interactSelectors := []string{
		".interact-info",
		".note-interact",
		".interact",
		".actions",
		".toolbar",
	}

	for _, selector := range interactSelectors {
		if elements, err := page.Elements(selector); err == nil && len(elements) > 0 {
			logrus.Infof("Found %d elements with selector: %s", len(elements), selector)
			for i, elem := range elements {
				if html, err := elem.HTML(); err == nil {
					logrus.Infof("Element %d HTML: %s", i, html[:min(200, len(html))])
				}
			}
		}
	}

	// 查找所有包含 "like" 的元素
	likeElements, err := page.Elements("[class*='like']")
	if err == nil {
		logrus.Infof("Found %d elements with 'like' in class", len(likeElements))
		for i, elem := range likeElements {
			if className, err := elem.Attribute("class"); err == nil {
				logrus.Infof("Like element %d class: %s", i, *className)
			}
		}
	}

	// 查找所有按钮元素
	buttons, err := page.Elements("button")
	if err == nil {
		logrus.Infof("Found %d button elements", len(buttons))
		for i, button := range buttons {
			if className, err := button.Attribute("class"); err == nil {
				logrus.Infof("Button %d class: %s", i, *className)
			}
		}
	}

	logrus.Info("=== End debugging ===")
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
