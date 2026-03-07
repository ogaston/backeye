FROM python:3.9-slim
RUN pip install ultralytics flask opencv-python
COPY tracker.py .
CMD ["python", "tracker.py"]