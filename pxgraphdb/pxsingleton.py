class PXSingleton (type):
    _instances = {}
    def __call__(cls, *args, **kwargs):
        if cls not in cls._instances:
            cls._instances[cls] = super(PXSingleton, cls).__call__(*args, **kwargs)
        return cls._instances[cls]

class PXSingletonDB (type):
    _instances = {}
    def __call__(cls, *args, **kwargs):
        if cls not in cls._instances:
            cls._instances[cls] = super(PXSingletonDB, cls).__call__(*args, **kwargs)
        return cls._instances[cls]
