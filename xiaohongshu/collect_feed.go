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

// CollectFeedAction 表示 Feed 收藏动作
type CollectFeedAction struct {
	page *rod.Page
}

// NewCollectFeedAction 创建 Feed 收藏动作
func NewCollectFeedAction(page *rod.Page) *CollectFeedAction {
	return &CollectFeedAction{page: page}
}

// CollectPost 收藏或取消收藏 Feed
func (c *CollectFeedAction) CollectPost(ctx context.Context, feedID, xsecToken string) (*CollectResult, error) {
	page := c.page.Context(ctx).Timeout(60 * time.Second)

	// 构建详情页 URL
	url := makeFeedDetailURL(feedID, xsecToken)

	logrus.Infof("Opening feed detail page for collect action: %s", url)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()

	time.Sleep(2 * time.Second)

	// 获取当前收藏状态（从页面数据获取）
	currentCollected, currentCount, err := c.getCurrentCollectStatus(page)
	if err != nil {
		logrus.Warnf("Failed to get current collect status: %v", err)
		// 设置默认值
		currentCollected = false
		currentCount = "0"
	}

	logrus.Infof("Current collect status - Collected: %v, Count: %s", currentCollected, currentCount)

	// 使用更精确的选择器策略，类似于like功能
	var collectButton *rod.Element
	
	// 首先尝试最常见的小红书收藏按钮选择器
	specificSelectors := []string{
		// 小红书特定的收藏按钮结构
		".interact-container .collect-wrapper",
		".interact-container .collect-btn",
		".interact-container button[class*='collect']",
		".interact-container div[class*='collect']",
		// 基于交互区域的精确查找
		".note-interact .collect-wrapper",
		".note-interact .collect-btn", 
		".note-interact button:nth-child(3)",  // 收藏通常是第三个按钮
		".note-interact div:nth-child(3)",
		// 更通用但仍然精确的选择器
		".interact-info button:nth-child(3)",
		".interact-info div:nth-child(3)",
	}

	for _, selector := range specificSelectors {
		if element, err := page.Element(selector); err == nil {
			collectButton = element
			logrus.Infof("Found collect button with selector: %s", selector)
			break
		}
	}

	// 如果精确选择器失败，尝试更广泛的搜索
	if collectButton == nil {
		var err error
		collectButton, err = c.findCollectButton(page)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find collect button")
		}
	}

	// 点击收藏按钮
	if err := collectButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, errors.Wrap(err, "failed to click collect button")
	}

	// 等待页面更新
	time.Sleep(3 * time.Second)

	// 获取更新后的收藏状态
	newCollected, newCount, err := c.getCurrentCollectStatus(page)
	if err != nil {
		logrus.Warnf("Failed to get updated collect status: %v", err)
		// 如果无法获取新状态，假设操作成功并切换状态
		newCollected = !currentCollected
		newCount = currentCount
	}

	logrus.Infof("Updated collect status - Collected: %v, Count: %s", newCollected, newCount)

	return &CollectResult{
		Collected:    newCollected,
		CollectCount: newCount,
		Action:       c.getActionType(currentCollected, newCollected),
	}, nil
}

// CollectResult 收藏操作结果
type CollectResult struct {
	Collected    bool   `json:"collected"`
	CollectCount string `json:"collect_count"`
	Action       string `json:"action"` // "collected" or "uncollected"
}

// findCollectButton 查找收藏按钮
func (c *CollectFeedAction) findCollectButton(page *rod.Page) (*rod.Element, error) {
	// 设置超时时间，避免无限等待
	timeout := time.After(30 * time.Second)

	// 按优先级排序的选择器列表
	prioritizedSelectors := []string{
		// 最高优先级：小红书特定的交互容器选择器
		".interact-container .collect-wrapper",
		".interact-container .collect-btn",
		".interact-container button[class*='collect']",
		".interact-container div[class*='collect']",
		
		// 高优先级：基于交互区域的精确查找
		".note-interact .collect-wrapper",
		".note-interact .collect-btn",
		".note-interact button:nth-child(3)",  // 收藏通常是第三个按钮
		".note-interact div:nth-child(3)",
		".interact-info button:nth-child(3)", 
		".interact-info div:nth-child(3)",
		
		// 中等优先级：小红书特定的收藏按钮选择器
		"button[class*='collect-lottie']",
		"button[class*='collect']",
		"div[class*='collect-lottie']", 
		"div[class*='collect']",
		"span[class*='collect-lottie']",
		"span[class*='collect']",
		
		// 较低优先级：基于交互区域的通用查找
		".interact-info button[class*='collect']",
		".interact-info div[class*='collect']",
		".note-interact button[class*='collect']",
		".note-interact div[class*='collect']",
		
		// 基于图标的查找
		"button svg[class*='star']",
		"div svg[class*='star']",
		"span svg[class*='star']",
		"button svg[class*='bookmark']",
		"div svg[class*='bookmark']",
		"span svg[class*='bookmark']",
		
		// 通用选择器
		".collect-btn",
		".collect-button", 
		"[data-testid*='collect']",
		"[data-testid*='bookmark']",
		
		// 基于文本的查找（最后尝试，因为可能不稳定）
		"button:contains('收藏')",
		"div:contains('收藏')",
		"span:contains('收藏')",
	}

	for i, selector := range prioritizedSelectors {
		select {
		case <-timeout:
			return nil, errors.New("timeout while searching for collect button")
		default:
			if element, err := page.Element(selector); err == nil {
				logrus.Infof("Found collect button with selector (priority %d): %s", i+1, selector)
				
				// 验证元素是否可点击
				if c.isElementClickable(element) {
					return element, nil
				} else {
					logrus.Warnf("Element found but not clickable with selector: %s", selector)
					continue
				}
			}
		}
	}

	// 如果所有选择器都失败，尝试调试页面结构
	logrus.Warn("All collect button selectors failed, attempting to debug page structure")
	c.debugPageStructure(page)

	return nil, errors.New("collect button not found with any known selector")
}

// isElementClickable 检查元素是否可点击
func (c *CollectFeedAction) isElementClickable(element *rod.Element) bool {
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

// getCurrentCollectStatus 获取当前收藏状态
func (c *CollectFeedAction) getCurrentCollectStatus(page *rod.Page) (bool, string, error) {
	// 方法1: 从 __INITIAL_STATE__ 获取（最可靠的方法）
	result := page.MustEval(`() => {
		try {
			if (window.__INITIAL_STATE__ && window.__INITIAL_STATE__.note) {
				const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
				for (const key in noteDetailMap) {
					const noteData = noteDetailMap[key];
					if (noteData && noteData.note && noteData.note.interactInfo) {
						return JSON.stringify({
							collected: noteData.note.interactInfo.collected,
							count: noteData.note.interactInfo.collectedCount || "0"
						});
					}
				}
			}
			return null;
		} catch (e) {
			console.error("Error getting collect status:", e);
			return null;
		}
	}`).String()

	if result != "null" && result != "" && result != "undefined" {
		// 尝试解析JSON结果
		var data struct {
			Collected bool   `json:"collected"`
			Count     string `json:"count"`
		}
		if err := json.Unmarshal([]byte(result), &data); err == nil {
			logrus.Infof("Got collect status from __INITIAL_STATE__: collected=%v, count=%s", data.Collected, data.Count)
			return data.Collected, data.Count, nil
		}
	}

	logrus.Warn("Failed to get collect status from __INITIAL_STATE__, trying DOM methods")

	// 方法2: 从DOM元素获取收藏状态
	// 检查收藏按钮是否有激活状态的class
	collectStatusSelectors := []string{
		// 小红书特定的激活状态选择器
		".interact-container .collect-wrapper.active",
		".interact-container .collect-btn.active", 
		".interact-container button[class*='collect'][class*='active']",
		".interact-container div[class*='collect'][class*='active']",
		".note-interact .collect-wrapper.active",
		".note-interact .collect-btn.active",
		".interact-info button[class*='active']:nth-child(3)",
		".interact-info div[class*='active']:nth-child(3)",
		// 通用激活状态选择器
		"button[class*='collect'][class*='active']",
		"div[class*='collect'][class*='active']",
		"span[class*='collect'][class*='active']",
		".collect-btn.active",
		".collect-button.active",
	}

	isCollected := false
	for _, selector := range collectStatusSelectors {
		if _, err := page.Element(selector); err == nil {
			isCollected = true
			logrus.Infof("Found active collect button with selector: %s", selector)
			break
		}
	}

	// 获取收藏数
	count := c.extractCollectCount(page)
	
	logrus.Infof("Got collect status from DOM: collected=%v, count=%s", isCollected, count)
	return isCollected, count, nil
}

// extractCollectCount 从页面中提取收藏数
func (c *CollectFeedAction) extractCollectCount(page *rod.Page) string {
	// 首先尝试从 __INITIAL_STATE__ 获取收藏数（最准确）
	result := page.MustEval(`() => {
		try {
			if (window.__INITIAL_STATE__ && window.__INITIAL_STATE__.note) {
				const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
				for (const key in noteDetailMap) {
					const noteData = noteDetailMap[key];
					if (noteData && noteData.note && noteData.note.interactInfo) {
						return noteData.note.interactInfo.collectedCount || "0";
					}
				}
			}
			return "0";
		} catch (e) {
			console.error("Error getting collect count:", e);
			return "0";
		}
	}`).String()

	if result != "" && result != "null" && result != "undefined" {
		logrus.Infof("Got collect count from __INITIAL_STATE__: %s", result)
		return result
	}

	// 如果从 __INITIAL_STATE__ 获取失败，尝试从DOM获取
	countSelectors := []string{
		// 小红书特定的收藏数选择器
		".interact-container .collect-wrapper .count",
		".interact-container .collect-btn .count",
		".interact-container button[class*='collect'] .count",
		".interact-container div[class*='collect'] .count",
		".note-interact .collect-wrapper .count",
		".note-interact .collect-btn .count",
		".interact-info button:nth-child(3) .count",
		".interact-info div:nth-child(3) .count",
		// 通用收藏数选择器
		".collect-count",
		".collect-count span",
		"[class*='collect'][class*='count']",
		".interact-info .collect-count",
		".note-interact .collect-count",
		// 更广泛的搜索
		"button[class*='collect'] span",
		"div[class*='collect'] span",
	}

	for _, selector := range countSelectors {
		if element, err := page.Element(selector); err == nil {
			if text, err := element.Text(); err == nil && text != "" {
				logrus.Infof("Got collect count from DOM selector %s: %s", selector, text)
				return text
			}
		}
	}

	logrus.Warn("Could not extract collect count, returning default value")
	return "0"
}

// getActionType 获取操作类型
func (c *CollectFeedAction) getActionType(oldCollected, newCollected bool) string {
	if !oldCollected && newCollected {
		return "collected"
	} else if oldCollected && !newCollected {
		return "uncollected"
	}
	return "no_change"
}

// debugPageStructure 调试页面结构，帮助找到正确的选择器
func (c *CollectFeedAction) debugPageStructure(page *rod.Page) {
	logrus.Info("=== Debugging page structure for collect button ===")

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

	// 查找所有包含 "collect" 的元素
	collectElements, err := page.Elements("[class*='collect']")
	if err == nil {
		logrus.Infof("Found %d elements with 'collect' in class", len(collectElements))
		for i, elem := range collectElements {
			if className, err := elem.Attribute("class"); err == nil && className != nil {
				logrus.Infof("Collect element %d class: %s", i, *className)
			}
			
			// 检查元素的标签名
			if tagName, err := elem.Eval("() => this.tagName"); err == nil {
				logrus.Infof("Collect element %d tag: %s", i, tagName.Value)
			}
			
			// 检查是否可见和可点击
			if visible, err := elem.Visible(); err == nil {
				logrus.Infof("Collect element %d visible: %v", i, visible)
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

	// 查找所有div元素（可能是收藏按钮）
	divs, err := page.Elements("div[class*='collect'], div[class*='interact']")
	if err == nil {
		logrus.Infof("Found %d div elements with collect/interact in class", len(divs))
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