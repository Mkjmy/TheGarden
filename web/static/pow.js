async function calculatePoW(content, statusElem) {
    let nonce = 0;
    const encoder = new TextEncoder();
    while (true) {
        const data = encoder.encode(content + nonce);
        const hashBuffer = await crypto.subtle.digest('SHA-256', data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
        
        if (hashHex.startsWith('0000')) {
            return nonce;
        }
        nonce++;
        if (nonce % 2000 === 0) {
            statusElem.innerText = `Đang đào... đã kiểm tra ${nonce} mã`;
            await new Promise(r => setTimeout(r, 1));
        }
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('postForm');
    const submitBtn = document.getElementById('submitBtn');
    const powStatus = document.getElementById('powStatus');
    const powNonceInput = document.getElementById('powNonce');

    if (!form) return;

    form.onsubmit = async (e) => {
        if (powNonceInput.value) return true;
        e.preventDefault();
        
        const titleInput = document.querySelector('input[name="title"]');
        const contentInput = document.querySelector('textarea[name="content"]');
        
        if (!titleInput.value || !contentInput.value) {
            alert("Vui lòng điền tiêu đề và nội dung.");
            return;
        }

        submitBtn.disabled = true;
        submitBtn.innerText = "ĐANG ĐÀO (DIGGING)...";
        
        powStatus.innerText = "Đang giải bài toán chống Spam (PoW)...";
        const startTime = Date.now();
        
        const nonce = await calculatePoW(titleInput.value + contentInput.value, powStatus);
        
        const duration = ((Date.now() - startTime) / 1000).toFixed(2);
        
        powNonceInput.value = nonce;
        powStatus.innerText = `Đã giải xong (${duration}s)! Đang gửi bài...`;
        submitBtn.disabled = false;
        submitBtn.innerText = "ĐANG GỬI...";
        form.submit();
    };
});
