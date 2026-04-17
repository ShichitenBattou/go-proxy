class ApplicationError(Exception):
    pass


class UserNotAuthorizedError(ApplicationError):
    pass
