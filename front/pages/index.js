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