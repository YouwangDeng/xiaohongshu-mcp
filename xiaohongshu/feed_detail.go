package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// FeedDetailAction 表示 Feed 详情页动作
type FeedDetailAction struct {
	page *rod.Page
}

// NewFeedDetailAction 创建 Feed 详情页动作
func NewFeedDetailAction(page *rod.Page) *FeedDetailAction {
	return &FeedDetailAction{page: page}
}

// GetFeedDetail 获取 Feed 详情页数据
func (f *FeedDetailAction) GetFeedDetail(ctx context.Context, feedID, xsecToken string) (*FeedDetailResponse, error) {
	page := f.page.Context(ctx).Timeout(60 * time.Second)

	// 构建详情页 URL
	url := makeFeedDetailURL(feedID, xsecToken)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	// 获取 window.__INITIAL_STATE__.note.noteDetailMap[feedID] 并转换为 JSON 字符串
	// 直接提取特定 feedID 的数据，避免 Vue.js 响应式对象的循环引用
	result := page.MustEval(fmt.Sprintf(`() => {
		if (window.__INITIAL_STATE__ && 
			window.__INITIAL_STATE__.note && 
			window.__INITIAL_STATE__.note.noteDetailMap &&
			window.__INITIAL_STATE__.note.noteDetailMap["%s"]) {
			const noteDetail = window.__INITIAL_STATE__.note.noteDetailMap["%s"];
			return JSON.stringify({
				note: noteDetail.note,
				comments: noteDetail.comments
			});
		}
		return "";
	}`, feedID, feedID)).String()

	if result == "" {
		return nil, fmt.Errorf("feed detail not found for feedID: %s", feedID)
	}

	// 直接解析为 FeedDetailResponse
	var response FeedDetailResponse
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feed detail: %w", err)
	}

	return &response, nil
}

func makeFeedDetailURL(feedID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_feed", feedID, xsecToken)
}
