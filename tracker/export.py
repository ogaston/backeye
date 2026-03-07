from ultralytics import YOLO

# Load your trained model (YOLO11, YOLO26, etc.)
model = YOLO("yolov8n-face.pt") 

# Export to ONNX
# 'imgsz' should match what you intend to use in Go (e.g., 640x640)
model.export(format="onnx", imgsz=640)