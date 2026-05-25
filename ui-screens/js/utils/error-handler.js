export const UIErrorHandler = {
    notify(error){
        // 1. 統一記錄到主控台以便除錯
        console.error("[系統錯誤]", error);

        // 2. 根據錯誤類型進行分支處理
        if (error.message.includes("401") || error.message.includes("Unauthorized")) {
            alert("登入逾時，請重新登入");
            window.location.href = '../index.html'; // 統一導向登入頁 
            return;
        }

        if (error.message.includes("409")) {
            // 針對 409 衝突，可以呼叫特定的 UI 組件顯示詳細資訊 
            this.showDetailedModal("資料衝突", error.message);
            return;
        }

        // 3. 一般錯誤的預設處理方式
        // 取代原本散落在各函數中的 alert('儲存失敗：' + err.message)
        this.showToast(`❌ 操作失敗: ${error.message}`);
    },

    showToast(message) {
        // 封裝 DOM 操作，取代原本直接修改特定 div 的做法
        const feedback = document.getElementById('import-result') || document.body;
        feedback.textContent = message;
        feedback.style.color = '#e74c3c';
    },
    handleConflict(data){
        this.showDetailedModal("資料衝突", "發現資料衝突，請檢查後再試一次。");
    },
    showDetailedModal(title, message) {
        // 移除可能已存在的舊 modal
        const existing = document.querySelector('.error-detail-modal');
        if (existing) existing.remove();

        const modal = document.createElement('div');
        modal.className = 'error-detail-modal';
        modal.style.cssText = 'position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.6);display:flex;align-items:center;justify-content:center;z-index:1000;';
        modal.onclick = function(e) { if (e.target === this) this.remove(); };
        modal.innerHTML = `
            <div style="background:#16213e;border-radius:12px;padding:24px;max-width:500px;width:90%;max-height:80vh;overflow-y:auto;">
                <h3 style="margin-bottom:16px;color:#e74c3c;">⚠️ ${title}</h3>
                <p style="color:#ccc;margin-bottom:16px;white-space:pre-wrap;">${message}</p>
                <button class="btn btn-secondary" style="margin-top:8px;" onclick="this.closest('.error-detail-modal').remove()">關閉</button>
            </div>`;
        document.body.appendChild(modal);
    },

    handleAuthError(){
        alert("登入逾時，請重新登入");
        window.location.href = '../index.html';
    }
    
}
