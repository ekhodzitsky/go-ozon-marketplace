package outbox

import (
	"testing"

	"github.com/IBM/sarama"
	saramamocks "github.com/IBM/sarama/mocks"
	"github.com/stretchr/testify/assert"
)

func TestSaramaProducer_SendMessage_Success(t *testing.T) {
	mock := saramamocks.NewSyncProducer(t, nil)
	defer mock.Close()

	mock.ExpectSendMessageAndSucceed()

	p := &SaramaProducer{producer: mock}
	err := p.SendMessage("orders", []byte("key"), []byte("value"))
	assert.NoError(t, err)
}

func TestSaramaProducer_SendMessage_Error(t *testing.T) {
	mock := saramamocks.NewSyncProducer(t, nil)
	defer mock.Close()

	mock.ExpectSendMessageAndFail(sarama.ErrInvalidMessage)

	p := &SaramaProducer{producer: mock}
	err := p.SendMessage("orders", []byte("key"), []byte("value"))
	assert.ErrorIs(t, err, sarama.ErrInvalidMessage)
}

func TestSaramaProducer_Close(t *testing.T) {
	mock := saramamocks.NewSyncProducer(t, nil)
	p := &SaramaProducer{producer: mock}

	assert.NoError(t, p.Close())
}
