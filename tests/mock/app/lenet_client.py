from datetime import datetime
from mindspore_serving.client import Client
import threading
import time
import os
from threading import Thread, Lock

send_request = 0
complete_request = 0
error_request = 0
lock = Lock()

def read_images():
    """Read images for directory test_image"""
    images_buffer = []
    image_files = []
    asb_images_path = os.environ.get("ABS_IMAGES_PATH")
    for path, _, file_list in os.walk(asb_images_path):
        for file_name in file_list:
            image_file = os.path.join(path, file_name)
            image_files.append(image_file)
    for image_file in image_files:
        with open(image_file, "rb") as fp:
            images_buffer.append(fp.read())
    return images_buffer, image_files


def run_classify_top1(first=False):
    """Client for servable lenet and method classify_top1"""
    request_time = datetime.now()
    request_timestamp_ms = int(request_time.timestamp()*1000)
    if first:
        request_timestamp_str = request_time.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]
        print("首次执行 classify_top1, 当前时间戳（毫秒）: ", request_timestamp_str)

    target_host = "localhost:8080"
    direct_backend_host = os.environ.get("DIRECT_BACKEND_HOST")
    if direct_backend_host:
        target_host = direct_backend_host

    client = Client(target_host, "lenet", "classify_top1")
    instances = []
    images_buffer, image_files = read_images()
    for image in images_buffer:
        instances.append({"image": image})

    global send_request
    with lock:
        send_request += 1

    try:
        result = client.infer(instances)
    except:
        global error_request
        with lock:
            error_request += 1

    global complete_request
    with lock:
        complete_request += 1
    response_time = datetime.now()
    response_timestamp_ms = int(response_time.timestamp()*1000)
    if first:
        response_timestamp_str = response_time.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]
        print("首次完成 classify_top1 请求, 当前时间戳（毫秒）: ", response_timestamp_str, "耗时（毫秒）: ", response_timestamp_ms - request_timestamp_ms)
        print("根据后端算法模型对上传的图片进行排名的结果如下: ")
        for item, file in zip(result, image_files):
            print("传输图像: ", os.path.basename(file), " 排名结果: ", item['label'])


def dispatch_tasks():
    thread_num = 20
    thread_num_env = os.environ.get("THREAD_NUM")
    if not thread_num:
        print("没有设置线程个数， 使用默认值 20")
        thread_num = int(thread_num_env)

    for _ in range(0, thread_num):
        workerThread = threading.Thread(target=loop_run_request)
        workerThread.setDaemon(True)
        workerThread.start()

def loop_run_request():
    while True:
        run_classify_top1(False)


def print_stat(start_timestamp_ms):
    now_time = datetime.now()
    now_timestamp_str = now_time.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]
    now_timestamp_ms = int(now_time.timestamp()*1000)
    total_time_used_ms = now_timestamp_ms - start_timestamp_ms
    each_request_time_used_ms = total_time_used_ms / complete_request
    service_group = os.environ.get("SERVICE_GROUP")
    qps = round(1000/each_request_time_used_ms)
    running_millsecond = total_time_used_ms / 1000
    with lock:
        print("[", now_timestamp_str, "] 已运行 ", running_millsecond , " 秒，当前应用属于 service_group = ", service_group, " 一共发送 ", send_request, " 一共完成 ", complete_request, " 次请求，平均一次请求耗时（毫秒）: ", each_request_time_used_ms, ", QPS: ", qps, " 推理报错个数: ", error_request)

def loop_print():
    start_timestamp_ms = int(datetime.now().timestamp()*1000)
    while True:
        time.sleep(5)
        print_stat(start_timestamp_ms)

if __name__ == '__main__':
    run_classify_top1(first=True)

    thread1 = threading.Thread(target=dispatch_tasks)
    thread1.setDaemon(True)
    thread1.start()

    loop_print()

    print("所有线程结束")