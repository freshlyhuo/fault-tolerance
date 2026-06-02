//CRC-16/CCITT-FALSE校验算法
static int calculate_file_crc(file_session_t *session) {
    if (!session) return -1;

    int fd = open(session->filepath, O_RDONLY);
    if (fd < 0) {
        perror("open file fail");
        return -1;
    }

    uint16_t crc = 0xFFFF;
    uint8_t buffer[4096];
    ssize_t bytes_read;
    lseek(fd, 0, SEEK_SET);

    while ((bytes_read = read(fd, buffer, sizeof(buffer))) > 0) {
        for (int i = 0; i < bytes_read; i++) {
            crc ^= buffer[i] << 8;
            for (int j = 0; j < 8; j++) {
                if (crc & 0x8000) {
                    crc = (crc << 1) ^ 0x1021;
                } else {
                    crc <<= 1;
                }
            }
        }
    }

    close(fd);
    session->crc16 = crc;

    printf("file crc=0x%04X\n",session->crc16);


    return crc;
}