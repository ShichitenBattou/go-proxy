const BASE_URL = 'https://auth.local/api';

async function apiCall(endpointId) {
    const resEl = document.getElementById(`res-${endpointId}`);
    resEl.textContent = '...';

    try {
        let response;
        switch (endpointId) {
            case 'get-public-posts':
                response = await getPublicPosts();
                break;
            case 'get-posts':
                response = await getPosts();
                break;
            case 'get-post':
                response = await getPost();
                break;
            case 'post-posts':
                response = await createPost();
                break;
            case 'post-users':
                response = await createUser();
                break;
            case 'get-user':
                response = await getUser();
                break;
            case 'delete-user':
                response = await deleteUser();
                break;
            default:
                resEl.textContent = `Unknown endpoint: ${endpointId}`;
                return;
        }
        resEl.textContent = response;
    } catch (error) {
        resEl.textContent = `Error: ${error.message}`;
    }
}

async function renderResponse(response) {
    const status = `${response.status} ${response.statusText}`;
    if (response.status === 204) {
        return `${status}`;
    }
    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
        const data = await response.json();
        return `${status}\n\n${JSON.stringify(data, null, 2)}`;
    }
    const text = await response.text();
    return `${status}\n\n${text}`;
}

function buildQueryString(params) {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
        if (value !== '' && value !== null && value !== undefined) {
            query.append(key, value);
        }
    }
    const qs = query.toString();
    return qs ? `?${qs}` : '';
}

// GET /public/posts
async function getPublicPosts() {
    const authorId = document.getElementById('get-public-posts-author_id').value.trim();
    const limit = document.getElementById('get-public-posts-limit').value.trim();
    const offset = document.getElementById('get-public-posts-offset').value.trim();

    const qs = buildQueryString({ author_id: authorId, limit, offset });
    const response = await fetch(`${BASE_URL}/public/posts${qs}`, {
        credentials: 'include',
    });
    return renderResponse(response);
}

// GET /posts
async function getPosts() {
    const authorId = document.getElementById('get-posts-author_id').value.trim();
    const limit = document.getElementById('get-posts-limit').value.trim();
    const offset = document.getElementById('get-posts-offset').value.trim();

    const qs = buildQueryString({ author_id: authorId, limit, offset });
    const response = await fetch(`${BASE_URL}/posts${qs}`, {
        credentials: 'include',
    });
    return renderResponse(response);
}

// GET /posts/{post_id}
async function getPost() {
    const postId = document.getElementById('get-post-post_id').value.trim();
    const response = await fetch(`${BASE_URL}/posts/${postId}`, {
        credentials: 'include',
    });
    return renderResponse(response);
}

// POST /posts
async function createPost() {
    const title = document.getElementById('post-posts-title').value.trim();
    const body = document.getElementById('post-posts-body').value.trim();
    const tagsRaw = document.getElementById('post-posts-tags').value.trim();
    const tags = tagsRaw ? tagsRaw.split(',').map(t => t.trim()).filter(t => t) : [];

    const response = await fetch(`${BASE_URL}/posts`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, body, tags }),
    });
    return renderResponse(response);
}

// POST /users
async function createUser() {
    const keycloakSub = document.getElementById('post-users-keycloak_sub').value.trim();

    const response = await fetch(`${BASE_URL}/users`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ keycloak_sub: keycloakSub }),
    });
    return renderResponse(response);
}

// GET /users/{user_id}
async function getUser() {
    const userId = document.getElementById('get-user-user_id').value.trim();
    const response = await fetch(`${BASE_URL}/users/${userId}`, {
        credentials: 'include',
    });
    return renderResponse(response);
}

// DELETE /users/{user_id}
async function deleteUser() {
    const userId = document.getElementById('delete-user-user_id').value.trim();
    const response = await fetch(`${BASE_URL}/users/${userId}`, {
        method: 'DELETE',
        credentials: 'include',
    });
    return renderResponse(response);
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
            loggedOutEl.classList.add('hidden');
            loggedInEl.classList.remove('hidden');

            const userNameEl = document.getElementById('user-name');
            if (userNameEl && user.name) {
                userNameEl.textContent = user.name;
            } else if (userNameEl && user.email) {
                userNameEl.textContent = user.email;
            } else if (userNameEl && user.preferred_username) {
                userNameEl.textContent = user.preferred_username;
            }
        } else {
            loggedInEl.classList.add('hidden');
            loggedOutEl.classList.remove('hidden');
        }
    } catch (error) {
        console.error('Error checking auth status:', error);
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

document.addEventListener('DOMContentLoaded', () => {
    checkAuthStatus();

    const logoutButton = document.getElementById('logout');
    if (logoutButton) {
        logoutButton.addEventListener('click', logout);
    }
});
