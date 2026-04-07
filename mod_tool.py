import sqlite3
import sys
import os

DB_PATH = "garden.db"

def connect_db():
    if not os.path.exists(DB_PATH):
        print(f"LỖI: Không tìm thấy file database tại {DB_PATH}")
        sys.exit(1)
    return sqlite3.connect(DB_PATH)

def list_pending():
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("SELECT id, title, author_name, published_at, category, is_locked FROM threads WHERE published_at > datetime('now') ORDER BY published_at ASC")
    rows = cursor.fetchall()
    print("\n--- CÁC BÀI VIẾT ĐANG CHỜ (SCHEDULED) ---")
    for row in rows:
        status = "[LOCKED]" if row[5] else "[OPEN]"
        print(f"ID: {row[0]} | {status} | Cat: {row[4]} | Tiêu đề: {row[1]} | Tác giả: {row[2]} | Sẽ đăng: {row[3]}")
    conn.close()

def hide_thread(thread_id):
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("UPDATE threads SET is_hidden = 1 WHERE id = ?", (thread_id,))
    conn.commit()
    if cursor.rowcount > 0:
        print(f"THÀNH CÔNG: Đã ẩn bài viết ID {thread_id}")
    else:
        print(f"LỖI: Không tìm thấy bài viết ID {thread_id}")
    conn.close()

def move_thread(thread_id, new_category):
    categories = ['feed', 'odd', 'inner', 'thoughts', 'signal']
    if new_category not in categories:
        print(f"LỖI: Chuyên mục '{new_category}' không hợp lệ. Chọn: {categories}")
        return
    
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("UPDATE threads SET category = ?, is_locked = 1 WHERE id = ?", (new_category, thread_id))
    conn.commit()
    if cursor.rowcount > 0:
        print(f"THÀNH CÔNG: Đã chuyển bài ID {thread_id} sang '{new_category}' (Đã khóa quyền cộng đồng).")
    else:
        print(f"LỖI: Không tìm thấy bài viết ID {thread_id}")
    conn.close()

def unlock_thread(thread_id):
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("UPDATE threads SET is_locked = 0 WHERE id = ?", (thread_id,))
    conn.commit()
    if cursor.rowcount > 0:
        print(f"THÀNH CÔNG: Đã MỞ KHÓA bài ID {thread_id}. Cộng đồng có thể tự gắn nhãn lại.")
    else:
        print(f"LỖI: Không tìm thấy bài viết ID {thread_id}")
    conn.close()

def checkin():
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("UPDATE system_settings SET value = datetime('now') WHERE key = 'last_mod_checkin'")
    conn.commit()
    print("THÀNH CÔNG: Đã check-in! Khu vườn sẽ tươi tốt thêm ít nhất 7 ngày nữa.")
    conn.close()

def show_usage():
    print("""
CÔNG CỤ ĐIỀU PHỐI (MOD TOOL) V3:
python3 mod_tool.py checkin           : Điểm danh (Tránh Read-Only)
python3 mod_tool.py list              : Xem bài đang chờ đăng
python3 mod_tool.py hide <id>         : Ẩn (xóa) bài viết
python3 mod_tool.py move <id> <cat>   : Chuyển category & Khóa nhãn
python3 mod_tool.py unlock <id>       : Mở khóa nhãn
python3 mod_tool.py help              : Hiện hướng dẫn này
    """)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        show_usage()
        sys.exit(0)

    cmd = sys.argv[1].lower()

    if cmd == "checkin":
        checkin()
    elif cmd == "list":
        list_pending()
    elif cmd == "hide" and len(sys.argv) == 3:
        hide_thread(sys.argv[2])
    elif cmd == "move" and len(sys.argv) == 4:
        move_thread(sys.argv[2], sys.argv[3])
    elif cmd == "unlock" and len(sys.argv) == 3:
        unlock_thread(sys.argv[2])
    else:
        show_usage()
