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

# диапазоны желтого и пурпурного цветов
yellow_min = 25
yellow_max = 35
lower_yellow = np.array([yellow_min, 0, 0])
upper_yellow = np.array([yellow_max, 255, 255])

purple_min = 135
purple_max = 160
lower_purple = np.array([purple_min, 0, 0])
upper_purple = np.array([purple_max, 255, 255])

mask_yellow = cv2.inRange(image_hsv, lower_yellow, upper_yellow)
mask_purple = cv2.inRange(image_hsv, lower_purple, upper_purple)

image_hsv_new = image_hsv.copy()
h = image_hsv[:, :, 0].astype(float)

# замена желтого на пурпурный
h_yellow = h[mask_yellow > 0]
h_yellow_new = purple_min + (h_yellow - yellow_min) * (
    purple_max - purple_min
) / (yellow_max - yellow_min)
h[mask_yellow > 0] = h_yellow_new

# Обратная замена пурпурного на желтый
h_purple = h[mask_purple > 0]
h_purple_new = yellow_min + (h_purple - purple_min) * (
    yellow_max - yellow_min
) / (purple_max - purple_min)
h[mask_purple > 0] = h_purple_new

image_hsv_new[:, :, 0] = np.clip(h, 0, 179).astype(np.uint8)
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
cv2.imwrite("image_out_2.jpg", image_bgr_new)