import { FileExtension } from '@sourcegraph/wildcard'

interface FileInfo {
    extension: FileExtension
    isTest: boolean
}

export function getFileInfo(file: string, isDirectory: boolean): FileInfo {
    if (isDirectory) {
        return {
            extension: 'default' as FileExtension,
            isTest: false,
        }
    }

    const extension = file.split('.').at(-1) as FileExtension
    const isValidExtension = Object.values(FileExtension).includes(extension)

    if (extension && isValidExtension) {
        return {
            extension,
            isTest: containsTest(file),
        }
    }

    return {
        extension: 'default' as FileExtension,
        isTest: false,
    }
}

export function containsTest(file: string): boolean {
    const f = file.split('.')
    // To account for other test file path structures
    // adjust this regular expression.
    const isTest = /^(test|spec|tests)(\b|_)|(\b|_)(test|spec|tests)$/

    for (const i of f) {
        if (i === 'test') {
            return true
        }
        if (isTest.test(i)) {
            return true
        }
    }
    return false
}
