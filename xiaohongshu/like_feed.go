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

	// 获取当前点赞状态（从页面数据获取）
	currentLiked, currentCount, err := l.getCurrentLikeStatus(page)
	if err != nil {
		logrus.Warnf("Failed to get current like status: %v", err)
		// 设置默认值
		currentLiked = false
		currentCount = "0"
	}

	logrus.Infof("Current like status - Liked: %v, Count: %s", currentLiked, currentCount)

	// 使用更精确的选择器策略，类似于comment功能
	var likeButton *rod.Element
	
	// 首先尝试最常见的小红书点赞按钮选择器
	specificSelectors := []string{
		// 小红书特定的点赞按钮结构
		".interact-container .like-wrapper",
		".interact-container .like-btn",
		".interact-container button[class*='like']",
		".interact-container div[class*='like']",
		// 基于交互区域的精确查找
		".note-interact .like-wrapper",
		".note-interact .like-btn", 
		".note-interact button:first-child",
		".note-interact div:first-child",
		// 更通用但仍然精确的选择器
		".interact-info button:first-child",
		".interact-info div:first-child",
	}

	for _, selector := range specificSelectors {
		if element, err := page.Element(selector); err == nil {
			likeButton = element
			logrus.Infof("Found like button with selector: %s", selector)
			break
		}
	}

	// 如果精确选择器失败，尝试更广泛的搜索
	if likeButton == nil {
		var err error
		likeButton, err = l.findLikeButton(page)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find like button")
		}
	}

	// 点击点赞按钮
	if err := likeButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, errors.Wrap(err, "failed to click like button")
	}

	// 等待页面更新
	time.Sleep(3 * time.Second)

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

	// 按优先级排序的选择器列表
	prioritizedSelectors := []string{
		// 最高优先级：小红书特定的交互容器选择器
		".interact-container .like-wrapper",
		".interact-container .like-btn",
		".interact-container button[class*='like']",
		".interact-container div[class*='like']",
		
		// 高优先级：基于交互区域的精确查找
		".note-interact .like-wrapper",
		".note-interact .like-btn",
		".note-interact button:first-child",
		".note-interact div:first-child",
		".interact-info button:first-child", 
		".interact-info div:first-child",
		
		// 中等优先级：小红书特定的点赞按钮选择器
		"button[class*='like-lottie']",
		"button[class*='like']",
		"div[class*='like-lottie']", 
		"div[class*='like']",
		"span[class*='like-lottie']",
		"span[class*='like']",
		
		// 较低优先级：基于交互区域的通用查找
		".interact-info button[class*='like']",
		".interact-info div[class*='like']",
		".note-interact button[class*='like']",
		".note-interact div[class*='like']",
		
		// 基于图标的查找
		"button svg[class*='heart']",
		"div svg[class*='heart']",
		"span svg[class*='heart']",
		
		// 通用选择器
		".like-btn",
		".like-button", 
		"[data-testid*='like']",
		
		// 基于文本的查找（最后尝试，因为可能不稳定）
		"button:contains('点赞')",
		"div:contains('点赞')",
		"span:contains('点赞')",
	}

	for i, selector := range prioritizedSelectors {
		select {
		case <-timeout:
			return nil, errors.New("timeout while searching for like button")
		default:
			if element, err := page.Element(selector); err == nil {
				logrus.Infof("Found like button with selector (priority %d): %s", i+1, selector)
				
				// 验证元素是否可点击
				if l.isElementClickable(element) {
					return element, nil
				} else {
					logrus.Warnf("Element found but not clickable with selector: %s", selector)
					continue
				}
			}
		}
	}

	// 如果所有选择器都失败，尝试调试页面结构
	logrus.Warn("All like button selectors failed, attempting to debug page structure")
	l.debugPageStructure(page)

	return nil, errors.New("like button not found with any known selector")
}

// isElementClickable 检查元素是否可点击
func (l *LikeFeedAction) isElementClickable(element *rod.Element) bool {
	// 检查元素是否可见
	visible, err := element.Visible()
	if err != nil || !visible {
		return false
	}
	
	// 检查元素是否有有效的边界框
	shape, err := element.Shape()
	if err != nil {
		return false
	}
	
	// 获取边界框
	box := shape.Box()
	if box == nil || box.Width == 0 || box.Height == 0 {
		return false
	}
	
	return true
}

// getCurrentLikeStatus 获取当前点赞状态
func (l *LikeFeedAction) getCurrentLikeStatus(page *rod.Page) (bool, string, error) {
	// 方法1: 从 __INITIAL_STATE__ 获取（最可靠的方法）
	result := page.MustEval(`() => {
		try {
			if (window.__INITIAL_STATE__ && window.__INITIAL_STATE__.note) {
				const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
				for (const key in noteDetailMap) {
					const noteData = noteDetailMap[key];
					if (noteData && noteData.note && noteData.note.interactInfo) {
						return JSON.stringify({
							liked: noteData.note.interactInfo.liked,
							count: noteData.note.interactInfo.likedCount || "0"
						});
					}
				}
			}
			return null;
		} catch (e) {
			console.error("Error getting like status:", e);
			return null;
		}
	}`).String()

	if result != "null" && result != "" && result != "undefined" {
		// 尝试解析JSON结果
		var data struct {
			Liked bool   `json:"liked"`
			Count string `json:"count"`
		}
		if err := json.Unmarshal([]byte(result), &data); err == nil {
			logrus.Infof("Got like status from __INITIAL_STATE__: liked=%v, count=%s", data.Liked, data.Count)
			return data.Liked, data.Count, nil
		}
	}

	logrus.Warn("Failed to get like status from __INITIAL_STATE__, trying DOM methods")

	// 方法2: 从DOM元素获取点赞状态
	// 检查点赞按钮是否有激活状态的class
	likeStatusSelectors := []string{
		// 小红书特定的激活状态选择器
		".interact-container .like-wrapper.active",
		".interact-container .like-btn.active", 
		".interact-container button[class*='like'][class*='active']",
		".interact-container div[class*='like'][class*='active']",
		".note-interact .like-wrapper.active",
		".note-interact .like-btn.active",
		".interact-info button[class*='active']:first-child",
		".interact-info div[class*='active']:first-child",
		// 通用激活状态选择器
		"button[class*='like'][class*='active']",
		"div[class*='like'][class*='active']",
		"span[class*='like'][class*='active']",
		".like-btn.active",
		".like-button.active",
	}

	isLiked := false
	for _, selector := range likeStatusSelectors {
		if _, err := page.Element(selector); err == nil {
			isLiked = true
			logrus.Infof("Found active like button with selector: %s", selector)
			break
		}
	}

	// 获取点赞数
	count := l.extractLikeCount(page)
	
	logrus.Infof("Got like status from DOM: liked=%v, count=%s", isLiked, count)
	return isLiked, count, nil
}

// extractLikeCount 从页面中提取点赞数
func (l *LikeFeedAction) extractLikeCount(page *rod.Page) string {
	// 首先尝试从 __INITIAL_STATE__ 获取点赞数（最准确）
	result := page.MustEval(`() => {
		try {
			if (window.__INITIAL_STATE__ && window.__INITIAL_STATE__.note) {
				const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
				for (const key in noteDetailMap) {
					const noteData = noteDetailMap[key];
					if (noteData && noteData.note && noteData.note.interactInfo) {
						return noteData.note.interactInfo.likedCount || "0";
					}
				}
			}
			return "0";
		} catch (e) {
			console.error("Error getting like count:", e);
			return "0";
		}
	}`).String()

	if result != "" && result != "null" && result != "undefined" {
		logrus.Infof("Got like count from __INITIAL_STATE__: %s", result)
		return result
	}

	// 如果从 __INITIAL_STATE__ 获取失败，尝试从DOM获取
	countSelectors := []string{
		// 小红书特定的点赞数选择器
		".interact-container .like-wrapper .count",
		".interact-container .like-btn .count",
		".interact-container button[class*='like'] .count",
		".interact-container div[class*='like'] .count",
		".note-interact .like-wrapper .count",
		".note-interact .like-btn .count",
		".interact-info button:first-child .count",
		".interact-info div:first-child .count",
		// 通用点赞数选择器
		".like-count",
		".like-count span",
		"[class*='like'][class*='count']",
		".interact-info .like-count",
		".note-interact .like-count",
		// 更广泛的搜索
		"button[class*='like'] span",
		"div[class*='like'] span",
	}

	for _, selector := range countSelectors {
		if element, err := page.Element(selector); err == nil {
			if text, err := element.Text(); err == nil && text != "" {
				logrus.Infof("Got like count from DOM selector %s: %s", selector, text)
				return text
			}
		}
	}

	logrus.Warn("Could not extract like count, returning default value")
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

	// 首先检查页面URL和标题
	url := page.MustInfo().URL
	title := page.MustEval("() => document.title").String()
	logrus.Infof("Page URL: %s", url)
	logrus.Infof("Page Title: %s", title)

	// 查找所有可能的交互容器
	interactSelectors := []string{
		".interact-container",
		".interact-info", 
		".note-interact",
		".interact",
		".actions",
		".toolbar",
		".bottom-bar",
		".action-bar",
	}

	for _, selector := range interactSelectors {
		if elements, err := page.Elements(selector); err == nil && len(elements) > 0 {
			logrus.Infof("Found %d elements with selector: %s", len(elements), selector)
			for i, elem := range elements {
				if html, err := elem.HTML(); err == nil {
					htmlStr := html
					if len(htmlStr) > 300 {
						htmlStr = htmlStr[:300] + "..."
					}
					logrus.Infof("Element %d HTML: %s", i, htmlStr)
				}
				
				// 获取元素的class属性
				if className, err := elem.Attribute("class"); err == nil && className != nil {
					logrus.Infof("Element %d class: %s", i, *className)
				}
			}
		}
	}

	// 查找所有包含 "like" 的元素
	likeElements, err := page.Elements("[class*='like']")
	if err == nil {
		logrus.Infof("Found %d elements with 'like' in class", len(likeElements))
		for i, elem := range likeElements {
			if className, err := elem.Attribute("class"); err == nil && className != nil {
				logrus.Infof("Like element %d class: %s", i, *className)
			}
			
			// 检查元素的标签名
			if tagName, err := elem.Eval("() => this.tagName"); err == nil {
				logrus.Infof("Like element %d tag: %s", i, tagName.Value)
			}
			
			// 检查是否可见和可点击
			if visible, err := elem.Visible(); err == nil {
				logrus.Infof("Like element %d visible: %v", i, visible)
			}
		}
	}

	// 查找所有按钮元素
	buttons, err := page.Elements("button")
	if err == nil {
		logrus.Infof("Found %d button elements", len(buttons))
		for i, button := range buttons {
			if className, err := button.Attribute("class"); err == nil && className != nil {
				logrus.Infof("Button %d class: %s", i, *className)
			}
			
			// 获取按钮文本
			if text, err := button.Text(); err == nil && text != "" {
				logrus.Infof("Button %d text: %s", i, text)
			}
		}
	}

	// 查找所有div元素（可能是点赞按钮）
	divs, err := page.Elements("div[class*='like'], div[class*='interact']")
	if err == nil {
		logrus.Infof("Found %d div elements with like/interact in class", len(divs))
		for i, div := range divs {
			if className, err := div.Attribute("class"); err == nil && className != nil {
				logrus.Infof("Div %d class: %s", i, *className)
			}
		}
	}

	// 尝试获取页面的 __INITIAL_STATE__ 信息
	initialStateInfo := page.MustEval(`() => {
		try {
			if (window.__INITIAL_STATE__) {
				return "Found __INITIAL_STATE__";
			}
			return "No __INITIAL_STATE__ found";
		} catch (e) {
			return "Error accessing __INITIAL_STATE__: " + e.message;
		}
	}`).String()
	logrus.Infof("Initial state info: %s", initialStateInfo)

	logrus.Info("=== End debugging ===")
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
