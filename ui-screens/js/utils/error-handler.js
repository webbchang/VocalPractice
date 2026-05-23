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
    handleAuthError(){
        alert("登入逾時，請重新登入");
        window.location.href = '../index.html';
    }
    
}