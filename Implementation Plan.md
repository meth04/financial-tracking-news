# Kế hoạch hoàn thiện MVP - Dự án Financial News Intelligence Crawler

Dự án hiện tại đã có khung xương backend Go và giao diện React hoạt động tương đối đầy đủ. Tuy nhiên, để dự án trở thành một sản phẩm MVP hoàn hảo về chức năng và trải nghiệm người dùng (UX), cần giải quyết một số lỗi tiềm ẩn và bổ sung các cải tiến quan trọng.

## Các cải tiến chính đề xuất

1. **Sửa lỗi ánh xạ Prompt Template LLM (Lỗi nghiêm trọng)**:
   - **Hiện trạng**: Hàm `RenderPrompt` trong `internal/llm/llm.go` chỉ thay thế các từ khóa như `{{title}}`, `{{source}}`, `{{content}}`. Trong khi đó, tệp mẫu prompt `prompts/article_analysis_user_template.md` lại sử dụng các từ khóa khác như `{{source_name}}`, `{{source_credibility_score}}`, `{{content_text}}`, `{{canonical_url}}`, `{{fetched_at}}`. Do đó, LLM nhận được mẫu thô có chứa các từ khóa chưa thay thế thay vì nội dung bài báo thực tế.
   - **Giải pháp**: Cập nhật `RenderPrompt` để hỗ trợ ánh xạ đầy đủ tất cả các khóa xuất hiện trong tệp template.

2. **Phục vụ giao diện tĩnh trực tiếp từ Máy chủ Go**:
   - **Hiện trạng**: Người dùng phải chạy song song hai lệnh (một để chạy Go API server trên cổng 8080 và một chạy Vite frontend dev/preview trên cổng 5173). điều này gây khó khăn khi triển khai hoặc chạy thử nghiệm cục bộ.
   - **Giải pháp**: Tích hợp tính năng phục vụ tệp tĩnh từ thư mục `web/dist` trong API router của Go (sử dụng `http.FileServer` và hỗ trợ SPA routing dự phòng về `index.html`). Khi chạy `finnews server`, người dùng chỉ cần truy cập vào `http://localhost:8080` là có thể sử dụng toàn bộ hệ thống.

3. **Nâng cấp toàn diện giao diện UI/UX (Đẹp mắt, Cao cấp và Tiện dụng)**:
   - **Tự động thăm dò trạng thái (Auto-polling)**: Khi có công việc LLM đang xử lý (`stats.llm_pending > 0`), giao diện React sẽ tự động làm mới dữ liệu sau mỗi 3 giây. Người dùng bấm "Crawl now" sẽ thấy tiến độ giảm dần và bài báo mới xuất hiện tự động mà không cần tải lại trang thủ công.
   - **Thiết kế UI hiện đại & Premium**:
     - Nâng cấp tông màu tối cao cấp (Harmonious dark color palette) kết hợp các hiệu ứng hover mượt mà và bo góc tinh tế.
     - Nâng cấp hiển thị điểm số (Credibility, Importance, Novelty) bằng các thanh tiến trình trực quan.
     - Cải tiến ngăn chi tiết bài báo (Drawer): Chia tab rõ ràng (Tổng quan, Nội dung gốc, JSON thô), hiển thị tóm tắt song song Tiếng Anh và Tiếng Việt.
     - Thiết kế lại dòng thời gian sự kiện (Timeline) dạng biểu đồ dọc chuyên nghiệp, có màu sắc biểu thị độ quan trọng tăng/giảm.
     - Thêm các biểu tượng SVG mini cho các loại tin tức (Fed, Doanh thu, Vĩ mô...) giúp người dùng dễ dàng phân loại bằng mắt.

---

## Thay đổi đề xuất trong mã nguồn

### [Go Backend Engine]

#### [MODIFY] [llm.go](file:///c:/Users/nguye/Documents/financial-tracking-news/internal/llm/llm.go)
- Cập nhật hàm `RenderPrompt` để thêm ánh xạ đầy đủ:
  - `{{source_name}}` -> tên nguồn tin.
  - `{{source_credibility_score}}` -> điểm uy tín.
  - `{{canonical_url}}` -> link gốc bài báo.
  - `{{content_text}}` -> nội dung bài báo.
  - `{{fetched_at}}` -> thời gian cào tin.

#### [MODIFY] [api.go](file:///c:/Users/nguye/Documents/financial-tracking-news/internal/api/api.go)
- Nhập thêm thư viện `"os"` và cập nhật router chi.
- Bổ sung định tuyến phục vụ tệp tĩnh từ `web/dist`. Nếu tệp yêu cầu không có đuôi mở rộng (không phải asset tĩnh), router sẽ phục vụ `web/dist/index.html` nhằm hỗ trợ React router (SPA).

---

### [React Frontend Dashboard]

#### [MODIFY] [main.tsx](file:///c:/Users/nguye/Documents/financial-tracking-news/web/src/main.tsx)
- Bổ sung hiệu ứng `useEffect` tự động chạy làm mới (`refresh()`) sau mỗi 3 giây khi phát hiện `stats.llm_pending > 0`.
- Thiết kế lại các component chính:
  - **FilterBar**: Bố trí grid gọn gàng hơn, các nút chọn trực quan.
  - **Articles / Events list**: Bảng dữ liệu dense đẹp mắt, có hiệu ứng hover mượt, hiển thị các nhãn SVG mini hoặc màu sắc phân cấp độ quan trọng (Bullish - xanh lục, Bearish - đỏ, Mixed - vàng).
  - **Drawer (Article / Event Detail)**: Bố cục cột đôi hoặc tabbed view. Phân chia rõ ràng phần Tóm tắt Việt-Anh, danh sách Key Facts và New Information dạng danh sách có gạch đầu dòng cách điệu.
  - **EventDetail Timeline**: Cách điệu Timeline với đường chỉ nối dọc và nút chấm phát sáng theo độ quan trọng của bản cập nhật sự kiện.

#### [MODIFY] [styles.css](file:///c:/Users/nguye/Documents/financial-tracking-news/web/src/styles.css)
- Nâng cấp hệ thống biến màu sắc CSS (`:root`).
- Thêm hiệu ứng chuyển động mịn (transitions), thanh cuộn (custom scrollbars) tối giản.
- Tinh chỉnh khoảng cách dòng (padding/margin), chiều rộng và chiều cao tối ưu hóa cho màn hình rộng (dense format).

---

## Kế hoạch kiểm thử & Xác minh

### Kiểm thử tự động
- Chạy `go test ./...` để kiểm tra việc biên dịch và các bài kiểm tra logic hiện tại.
- Thêm test case cho `TestRenderPrompt` trong `internal/llm/llm_test.go` để đảm bảo tất cả từ khóa mới trong prompt template đều được phân giải chính xác.

### Kiểm thử thủ công
1. Xây dựng lại ứng dụng frontend:
   ```bash
   cd web
   npm run build
   ```
2. Chạy ứng dụng Go backend duy nhất:
   ```bash
   go run ./cmd/finnews migrate up
   go run ./cmd/finnews seed sources
   go run ./cmd/finnews server
   ```
3. Mở trình duyệt tại địa chỉ `http://localhost:8080` (thay vì 5173) để xác nhận:
   - Giao diện người dùng tải lên bình thường và giao tiếp tốt với API.
   - Nhấp vào "Crawl now" và quan sát số lượng "LLM pending" tăng lên trong stats pill, giao diện tự động cập nhật mà không cần F5.
   - Kiểm tra Drawer chi tiết bài báo xem nội dung bài báo đã được hiển thị chuẩn xác (đã sửa lỗi prompt thô).
   - Kiểm tra Timeline của Sự kiện hoạt động đúng.
