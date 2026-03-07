import cv2
import socket
import threading
import sys
import time
from ultralytics import YOLO

event_to_send = None
event_lock = threading.Lock()
stop_event = threading.Event()

def tcp_sender_thread(host, port):
    global event_to_send

    server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    server_socket.bind((host, port))
    server_socket.listen(1)
    server_socket.settimeout(1.0)

    print(f"[TCP] Server listening on {port}...")

    while not stop_event.is_set():
        client_socket = None
        try:
            client_socket, addr = server_socket.accept()
            print(f"[TCP] Client connected: {addr}")

            while not stop_event.is_set():
                with event_lock:
                    msg = event_to_send
                    event_to_send = None

                if msg is None:
                    time.sleep(0.01)
                    continue

                client_socket.sendall((msg + "\n").encode())

        except socket.timeout:
            continue
        except (ConnectionResetError, BrokenPipeError):
            print("[TCP] Client disconnected.")
        except Exception as e:
            if not stop_event.is_set():
                print(f"[TCP] Error: {e}")
        finally:
            if client_socket:
                client_socket.close()

    server_socket.close()
    print("[TCP] Thread exiting.")

def main():
    global event_to_send
    model = YOLO('yolov8n-face.pt')
    cap = cv2.VideoCapture(0)

    network_thread = threading.Thread(target=tcp_sender_thread, args=('0.0.0.0', 8089), daemon=True)
    network_thread.start()

    print("Starting Main Loop... Press 'q' or Ctrl+C to exit.")

    try:
        while True:
            ret, frame = cap.read()
            if not ret:
                break

            results = model(frame, verbose=False)
            face_count = 0

            for result in results:
                face_count += len(result.boxes)
                for box in result.boxes:
                    x1, y1, x2, y2 = map(int, box.xyxy[0])
                    cv2.rectangle(frame, (x1, y1), (x2, y2), (0, 255, 0), 2)

            if face_count > 0:
                with event_lock:
                    event_to_send = f"FACE_DETECTED count={face_count}"

            # Local Preview (runs at full FPS)
            cv2.imshow('Face Tracking (High Speed)', frame)
            
            if cv2.waitKey(1) & 0xFF == ord('q'):
                break

    except KeyboardInterrupt:
        print("\n[!] Ctrl+C detected.")
    finally:
        print("Cleaning up...")
        stop_event.set()
        cap.release()
        cv2.destroyAllWindows()
        time.sleep(0.5)
        print("Shutdown complete.")
        sys.exit(0)

if __name__ == "__main__":
    main()