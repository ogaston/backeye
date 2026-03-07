from flask import Flask, jsonify
import cv2
from ultralytics import YOLO
import threading
import time
import os
import socket
import json

app = Flask(__name__)


model = YOLO('yolov8n-face.pt')

latest_detection = {"faces": [], "timestamp": 0}

def run_flask():
    # Run Flask in a separate thread
    app.run(host='0.0.0.0', port=5000, debug=False, use_reloader=False)

@app.route('/detections', methods=['GET'])
def get_detections():
    return jsonify(latest_detection)

def track_faces():
    global latest_detection
    cap = cv2.VideoCapture(0)
    
    # Check if camera opened successfully
    if not cap.isOpened():
        print("Error: Could not open webcam.")
        return

    print("Starting detection... Press 'q' in the window to quit.")
    
    while True:
        ret, frame = cap.read()
        if not ret:
            break
        
        results = model(frame, verbose=False)
        faces = []
        
        for result in results:
            for box in result.boxes:
                # Extract coordinates and confidence
                x1, y1, x2, y2 = map(int, box.xyxy[0])
                conf = float(box.conf[0])
                
                faces.append({
                    "confidence": conf,
                    "bbox": [x1, y1, x2, y2]
                })
                
                # Visual Feedback
                cv2.rectangle(frame, (x1, y1), (x2, y2), (0, 255, 0), 2)
                cv2.putText(frame, f'Face {conf:.2f}', (x1, y1 - 10),
                            cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 255, 0), 2)
        
        # Update the global state for the API
        latest_detection = {
            "faces": faces,
            "timestamp": time.time()
        }
        
        cv2.imshow('Face Tracking', frame)
        if cv2.waitKey(1) & 0xFF == ord('q'):
            break
    
    cap.release()
    cv2.destroyAllWindows()

def main():


    # # 1. Start Flask in the background
    threading.Thread(target=run_flask, daemon=True).start()
    
    # # 2. Run OpenCV in the main thread
    track_faces()

if __name__ == '__main__':
    main()