import numpy as np
import cv2
import matplotlib.pyplot as plt

image_path = input("Путь к изображению: ")

image_in = cv2.imread(image_path)

if image_in is None:
    print("Ошибка: изображение не найдено.")
    exit()

image_rgb = cv2.cvtColor(image_in, cv2.COLOR_BGR2RGB)
image_hsv = cv2.cvtColor(image_rgb, cv2.COLOR_RGB2HSV)

lower_red1 = np.array([0, 0, 0])
upper_red1 = np.array([10, 255, 255])

lower_red2 = np.array([170, 0, 0])
upper_red2 = np.array([179, 255, 255])

mask_red1 = cv2.inRange(image_hsv, lower_red1, upper_red1)
mask_red2 = cv2.inRange(image_hsv, lower_red2, upper_red2)
mask_red = mask_red1 | mask_red2

image_hsv_new = image_hsv.copy()
image_hsv_new[mask_red > 0, 1] = 0
image_rgb_new = cv2.cvtColor(image_hsv_new, cv2.COLOR_HSV2RGB)

plt.figure(figsize=(10, 5))
plt.subplot(1, 2, 1)
plt.title('Исходное изображение')
plt.imshow(image_rgb)
plt.axis('off')

plt.subplot(1, 2, 2)
plt.title('Преобразованное изображение')
plt.imshow(image_rgb_new)
plt.axis('off')
plt.show()

image_bgr_new = cv2.cvtColor(image_rgb_new, cv2.COLOR_RGB2BGR)
cv2.imwrite("image_out_1.jpg", image_bgr_new)