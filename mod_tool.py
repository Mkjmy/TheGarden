import psycopg2
import sys
import os
from urllib.parse import urlparse

def connect_db():
    db_url = os.getenv("DATABASE_URL", "postgres://garden:garden@localhost:5432/garden?sslmode=disable")
    try:
        conn = psycopg2.connect(db_url)
        return conn
    except Exception as e:
        print(f"ERROR: Could not connect to Database. {e}")
        sys.exit(1)

def list_pending():
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("SELECT id, title, author_name, published_at, category, is_locked FROM threads WHERE published_at > CURRENT_TIMESTAMP ORDER BY published_at ASC")
    rows = cursor.fetchall()
    print("\n--- PENDING POSTS (SCHEDULED) ---")
    for row in rows:
        status = "[LOCKED]" if row[5] else "[OPEN]"
        print(f"ID: {row[0]} | {status} | Cat: {row[4]} | Title: {row[1]} | Author: {row[2]} | Will publish: {row[3]}")
    conn.close()

def hide_thread(thread_id):
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("UPDATE threads SET is_hidden = TRUE WHERE id = %s", (thread_id,))
    conn.commit()
    if cursor.rowcount > 0:
        print(f"SUCCESS: Hidden post ID {thread_id}")
    else:
        print(f"ERROR: Could not find post ID {thread_id}")
    conn.close()

def move_thread(thread_id, new_category):
    categories = ['feed', 'odd', 'inner', 'thoughts', 'signal']
    if new_category not in categories:
        print(f"ERROR: Category '{new_category}' is invalid. Choose from: {categories}")
        return
    
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("UPDATE threads SET category = %s, is_locked = TRUE WHERE id = %s", (new_category, thread_id))
    conn.commit()
    if cursor.rowcount > 0:
        print(f"SUCCESS: Moved post ID {thread_id} to '{new_category}' (Community tagging locked).")
    else:
        print(f"ERROR: Could not find post ID {thread_id}")
    conn.close()

def unlock_thread(thread_id):
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("UPDATE threads SET is_locked = FALSE WHERE id = %s", (thread_id,))
    conn.commit()
    if cursor.rowcount > 0:
        print(f"SUCCESS: UNLOCKED post ID {thread_id}. Community can now re-tag it.")
    else:
        print(f"ERROR: Could not find post ID {thread_id}")
    conn.close()

def checkin():
    conn = connect_db()
    cursor = conn.cursor()
    cursor.execute("UPDATE system_settings SET value = TO_CHAR(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS') WHERE key = 'last_mod_checkin'")
    conn.commit()
    print("SUCCESS: Checked-in! The Garden will remain active for at least 7 more days.")
    conn.close()

def show_usage():
    print("""
MODERATION TOOL V3:
python3 mod_tool.py checkin           : Check-in (Avoid Read-Only)
python3 mod_tool.py list              : View pending posts
python3 mod_tool.py hide <id>         : Hide (delete) post
python3 mod_tool.py move <id> <cat>   : Change category & Lock tagging
python3 mod_tool.py unlock <id>       : Unlock tagging
python3 mod_tool.py help              : Show this help guide
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
