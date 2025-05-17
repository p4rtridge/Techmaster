import sys
import json
import os
from PIL import Image, ImageOps, ImageEnhance
import numpy as np
from paddleocr import PaddleOCR

def detect_text_color(image_path):
    """
    Phát hiện xem nét chữ trong ảnh chủ yếu là màu tối hay màu sáng
    Trả về True nếu chữ chủ yếu là màu sáng (cần đảo ngược)
    """
    try:
        # Mở ảnh
        img = Image.open(image_path).convert('L')  # Chuyển sang ảnh xám
        img_array = np.array(img)
        
        # Tính toán histogram
        hist = np.histogram(img_array, bins=256, range=(0, 256))[0]
        
        # Tính tỷ lệ pixel tối và sáng
        dark_pixels = np.sum(hist[:128])  # Pixel tối (0-127)
        light_pixels = np.sum(hist[128:])  # Pixel sáng (128-255)
        
        # Nếu pixel sáng nhiều hơn, có thể đây là chữ tối trên nền sáng (không cần đảo ngược)
        # Nếu pixel tối nhiều hơn, có thể đây là chữ sáng trên nền tối (cần đảo ngược)
        return light_pixels < dark_pixels
    except Exception as e:
        print(f"Warning: Error detecting text color: {str(e)}", file=sys.stderr)
        return False  # Mặc định không đảo ngược

def preprocess_image(image_path, max_width=1600, max_height=1600):
    """
    Tiền xử lý ảnh:
    1. Resize nếu cần
    2. Phát hiện màu chữ và đảo ngược màu nếu cần
    3. Tăng cường độ tương phản
    """
    try:
        # Mở ảnh
        img = Image.open(image_path)
        
        # Resize nếu cần
        width, height = img.size
        if width > max_width or height > max_height:
            ratio = min(max_width / width, max_height / height)
            new_width = int(width * ratio)
            new_height = int(height * ratio)
            img = img.resize((new_width, new_height), Image.LANCZOS)
        
        # Phát hiện màu chữ
        invert_needed = detect_text_color(image_path)
        
        # Tạo hai phiên bản của ảnh: bình thường và đảo ngược
        # Cả hai sẽ được gửi vào OCR để tăng khả năng nhận diện
        
        # Phiên bản 1: Tăng độ tương phản
        img_enhanced = ImageEnhance.Contrast(img).enhance(2.0)
        
        # Phiên bản 2: Đảo ngược màu và tăng độ tương phản
        img_inverted = ImageOps.invert(img.convert('RGB'))
        img_inverted_enhanced = ImageEnhance.Contrast(img_inverted).enhance(2.0)
        
        # Lưu các phiên bản ảnh
        file_name, file_ext = os.path.splitext(image_path)
        enhanced_path = f"{file_name}_enhanced{file_ext}"
        inverted_path = f"{file_name}_inverted{file_ext}"
        
        img_enhanced.save(enhanced_path, quality=95, optimize=True)
        img_inverted_enhanced.save(inverted_path, quality=95, optimize=True)
        
        return enhanced_path, inverted_path, invert_needed
    except Exception as e:
        print(json.dumps([{"error": f"Error preprocessing image: {str(e)}"}]), file=sys.stderr)
        return image_path, None, False

def process_image(image_path, max_width=1600, max_height=1600):
    try:
        # Tiền xử lý ảnh
        enhanced_path, inverted_path, invert_needed = preprocess_image(image_path, max_width, max_height)
        
        # Khởi tạo PaddleOCR với language model
        ocr = PaddleOCR(use_angle_cls=True, lang='ch', show_log=False, use_gpu=False)
        
        # Thử OCR trên cả hai phiên bản ảnh
        result_enhanced = ocr.ocr(enhanced_path, cls=True)
        result_inverted = None
        if inverted_path:
            result_inverted = ocr.ocr(inverted_path, cls=True)
        
        # Xóa file tạm
        if enhanced_path != image_path and os.path.exists(enhanced_path):
            os.remove(enhanced_path)
        if inverted_path and os.path.exists(inverted_path):
            os.remove(inverted_path)
        
        # Chọn kết quả tốt nhất
        # Nếu ảnh cần đảo ngược màu và ảnh đảo ngược cho nhiều kết quả hơn, dùng kết quả đó
        result = None
        if result_enhanced and result_enhanced[0] and (not result_inverted or not result_inverted[0]):
            result = result_enhanced
        elif result_inverted and result_inverted[0] and (not result_enhanced or not result_enhanced[0]):
            result = result_inverted
        elif result_enhanced and result_inverted and result_enhanced[0] and result_inverted[0]:
            # So sánh số lượng kết quả và độ tin cậy
            count_enhanced = len(result_enhanced[0])
            count_inverted = len(result_inverted[0])
            
            conf_enhanced = sum(line[1][1] for line in result_enhanced[0]) if count_enhanced > 0 else 0
            conf_inverted = sum(line[1][1] for line in result_inverted[0]) if count_inverted > 0 else 0
            
            # Nếu là chữ sáng trên nền tối, ưu tiên kết quả từ ảnh đảo ngược
            if invert_needed and count_inverted > 0:
                result = result_inverted
            # Ngược lại, chọn kết quả có nhiều phát hiện hơn hoặc độ tin cậy cao hơn
            elif count_inverted > count_enhanced:
                result = result_inverted
            elif count_enhanced > count_inverted:
                result = result_enhanced
            elif conf_inverted > conf_enhanced:
                result = result_inverted
            else:
                result = result_enhanced
        else:
            # Nếu không có kết quả nào, sử dụng kết quả rỗng
            result = [None]
        
        # Chuyển đổi kết quả sang định dạng JSON
        json_result = []
        
        if result and result[0]:
            for line in result[0]:
                coords = line[0]
                text = line[1][0]
                confidence = line[1][1]
                
                json_result.append({
                    "coords": coords,
                    "text": text,
                    "confidence": float(confidence)
                })
        
        # In kết quả dưới dạng JSON
        print(json.dumps(json_result))
        
    except Exception as e:
        print(json.dumps([{"error": str(e)}]))

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(json.dumps([{"error": "No image path provided"}]))
        sys.exit(1)
    
    image_path = sys.argv[1]
    
    # Lấy kích thước max từ tham số nếu có
    max_width = 1600
    max_height = 1600
    
    if len(sys.argv) >= 3:
        try:
            max_width = int(sys.argv[2])
        except ValueError:
            pass
    
    if len(sys.argv) >= 4:
        try:
            max_height = int(sys.argv[3])
        except ValueError:
            pass
    
    process_image(image_path, max_width, max_height)