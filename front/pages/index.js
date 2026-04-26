async function getPosts() {
    try {
        const response = await fetch('https://auth.local/api/posts');
        const data = await response.json();
        console.log(data);
    } catch (error) {
        console.error('Error fetching posts:', error);
        return [];
    }
}

async function checkAuthStatus() {
    try {
        const response = await fetch('https://auth.local/api/auth/me', {
            credentials: 'include'
        });

        const loggedOutEl = document.querySelector('.auth-logged-out');
        const loggedInEl = document.querySelector('.auth-logged-in');

        if (response.ok) {
            const user = await response.json();
            // ログイン状態
            loggedOutEl.classList.add('hidden');
            loggedInEl.classList.remove('hidden');

            // ユーザー名を表示
            const userNameEl = document.getElementById('user-name');
            if (userNameEl && user.name) {
                userNameEl.textContent = user.name;
            } else if (userNameEl && user.email) {
                // name が取得できない場合は email を表示
                userNameEl.textContent = user.email;
            } else if (userNameEl && user.preferred_username) {
                // preferred_username を表示
                userNameEl.textContent = user.preferred_username;
            }
        } else {
            // ログアウト状態
            loggedInEl.classList.add('hidden');
            loggedOutEl.classList.remove('hidden');
        }
    } catch (error) {
        console.error('Error checking auth status:', error);
        // エラー時はログアウト状態として扱う
        const loggedOutEl = document.querySelector('.auth-logged-out');
        const loggedInEl = document.querySelector('.auth-logged-in');
        if (loggedInEl) loggedInEl.classList.add('hidden');
        if (loggedOutEl) loggedOutEl.classList.remove('hidden');
    }
}

async function logout() {
    try {
        const response = await fetch('https://auth.local/api/auth/logout', {
            method: 'POST',
            credentials: 'include'
        });

        if (response.ok) {
            // ログアウト成功時は UI を更新
            const loggedOutEl = document.querySelector('.auth-logged-out');
            const loggedInEl = document.querySelector('.auth-logged-in');
            loggedInEl.classList.add('hidden');
            loggedOutEl.classList.remove('hidden');
        } else {
            console.error('Logout failed:', response.status);
            alert('ログアウトに失敗しました');
        }
    } catch (error) {
        console.error('Error during logout:', error);
        alert('ログアウト中にエラーが発生しました');
    }
}

// 初期化処理
document.addEventListener('DOMContentLoaded', () => {
    // 認証状態をチェック
    checkAuthStatus();

    // ログアウトボタンのイベントリスナーを設定
    const logoutButton = document.getElementById('logout');
    if (logoutButton) {
        logoutButton.addEventListener('click', logout);
    }
});