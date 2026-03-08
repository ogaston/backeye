import cv2
import socket
import threading
import sys
import os
import time
from ultralytics import YOLO
from collections import deque

class NetworkService:
    """
    Serves event messages over a TCP socket.
    """
    def __init__(self, host="0.0.0.0", port=8089):
        self.host = host
        self.port = port
        
        self.stop_event = threading.Event()
        self.event_lock = threading.Lock()

        self.theard = threading.Thread(target=self._run_server, daemon=True)
        self.event_queue = deque(maxlen=50)
        self.connection_status = 'disconnected'

    def start(self):
        self.theard.start()

    def stop(self):
        self.stop_event.set()

    def get_connection_status(self):
        with self.event_lock:
            return self.connection_status
        return 'disconnected'

    def send_event(self, message):
        with self.event_lock:
            self.event_queue.append(message)

    def _run_server(self):
        server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server_socket.bind((self.host, self.port))
        server_socket.listen(1)
        server_socket.settimeout(1.0)

        print(f"[TRACKER] Server listening on {self.host}:{self.port}...")

        while not self.stop_event.is_set():
            client_socket = None
            try:
                client_socket, addr = server_socket.accept()
                print(f"[TRACKER] Client connected: {addr}")
                with self.event_lock:
                    self.connection_status = 'connected | client: ' + addr[0]

                while not self.stop_event.is_set():
                    try:
                        message = None
                        with self.event_lock:
                            if self.event_queue:
                                message = self.event_queue.popleft()
                        if message:
                            client_socket.sendall((message + "\n").encode())
                        else:
                            time.sleep(0.01)
                    except (ConnectionResetError, BrokenPipeError):
                        raise  # propagate so outer handler closes socket
                    except Exception as e:
                        print(f"[TRACKER] Error: {e}")
                        continue

            except socket.timeout:
                continue
            except (ConnectionResetError, BrokenPipeError):
                with self.event_lock:
                    self.connection_status = 'disconnected'
                print(f"[TRACKER] Client disconnected: {addr}")
            except Exception as e:
                print(f"[TRACKER] Error: {e}")
                if client_socket:
                    client_socket.close()
            finally:
                if client_socket:
                    client_socket.close()

        server_socket.close()
        print("[TRACKER] Server stopped")

class DetectionService:
    """
    Detects objects in a video stream and returns the number of objects detected.
    """
    def __init__(self, model_path="yolov8n-face.pt"):
        self.model = YOLO(model_path)
        self.cap = cv2.VideoCapture(0)

    def run(self):
        ret, frame = self.cap.read()
        if not ret:
            return None, 0

        results = self.model(frame, verbose=False)
        face_count = 0

        for result in results:
            face_count += len(result.boxes)
            for box in result.boxes:
                x1, y1, x2, y2 = map(int, box.xyxy[0])
                cv2.rectangle(frame, (x1, y1), (x2, y2), (0, 255, 0), 2)

        return frame, face_count

    def stop(self):
        self.cap.release()
        cv2.destroyAllWindows()

class LoggerService: 
    """
    Logs the status of the face tracking service to a file.
    """
    def __init__(self, log_file="face_tracking.log"):
        self.log_file = log_file
        self.last_print_time = 0
        self.update_interval = os.getenv('UPDATE_INTERVAL', 1.0)
        self._show_status_enabled = os.getenv('SHOW_STATUS', 'true')

    def log(self, message):
        with open(self.log_file, 'a') as f:
            f.write(message + '\n')

    def show_status(self, face_count, location, connection_status):
        current_time = time.time()
        if current_time - self.last_print_time >= self.update_interval:
            self.last_print_time = current_time
            if self._show_status_enabled == 'true':
                print("\033[H\033[J", end="") 
                print("   FACE TRACKING DASHBOARD    ")
                print("===============================")
                print(f" Status:   [{'DETECTING' if face_count > 0 else 'NO DETECTION'}]")
                print(f" Location: {location}")
                print(f" Faces:    {face_count}")
                print(f" Time:     {time.strftime('%H:%M:%S')}")
                print(f" Connection Status: {connection_status}")
                print("-------------------------------")
                print(" Press 'q' in the window to quit")

def main():
    # clean the screen
    os.system('clear')

    # environment variables
    show_window = os.getenv('SHOW_WINDOW', 'true')
    location = os.getenv('LOCATION', 'unknown')

    logger_service = LoggerService()
    detection_service = DetectionService()
    network_service = NetworkService()

    network_service.start() 
    print("[TRACKER] Network service started. press 'q' to exit...")

    try:
        while True:
            frame, face_count = detection_service.run()
            if frame is not None and show_window == 'true':
                cv2.imshow('Face Tracking (High Speed)', frame)
            
            network_service.send_event(str(face_count))

            logger_service.show_status(face_count, location, network_service.get_connection_status())

            # check for quit key
            if cv2.waitKey(1) & 0xFF == ord('q'):
                break
        
    except KeyboardInterrupt:
        print("\n[!] Ctrl+C detected.")
    finally:
        detection_service.stop()
        network_service.stop()
        print("[TRACKER] Detection service stopped.")
        print("[TRACKER] Network service stopped.")

if __name__ == "__main__":
    main()