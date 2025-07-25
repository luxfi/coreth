// Code generated by rlpgen. DO NOT EDIT.

package bal

import "github.com/luxfi/geth/rlp"
import "io"

func (obj *BlockAccessList) EncodeRLP(_w io.Writer) error {
	w := rlp.NewEncoderBuffer(_w)
	_tmp0 := w.List()
	_tmp1 := w.List()
	for _, _tmp2 := range obj.Accesses {
		_tmp3 := w.List()
		w.WriteBytes(_tmp2.Address[:])
		_tmp4 := w.List()
		for _, _tmp5 := range _tmp2.StorageWrites {
			_tmp6 := w.List()
			w.WriteBytes(_tmp5.Slot[:])
			_tmp7 := w.List()
			for _, _tmp8 := range _tmp5.Accesses {
				_tmp9 := w.List()
				w.WriteUint64(uint64(_tmp8.TxIdx))
				w.WriteBytes(_tmp8.ValueAfter[:])
				w.ListEnd(_tmp9)
			}
			w.ListEnd(_tmp7)
			w.ListEnd(_tmp6)
		}
		w.ListEnd(_tmp4)
		_tmp10 := w.List()
		for _, _tmp11 := range _tmp2.StorageReads {
			w.WriteBytes(_tmp11[:])
		}
		w.ListEnd(_tmp10)
		_tmp12 := w.List()
		for _, _tmp13 := range _tmp2.BalanceChanges {
			_tmp14 := w.List()
			w.WriteUint64(uint64(_tmp13.TxIdx))
			w.WriteBytes(_tmp13.Balance[:])
			w.ListEnd(_tmp14)
		}
		w.ListEnd(_tmp12)
		_tmp15 := w.List()
		for _, _tmp16 := range _tmp2.NonceChanges {
			_tmp17 := w.List()
			w.WriteUint64(uint64(_tmp16.TxIdx))
			w.WriteUint64(_tmp16.Nonce)
			w.ListEnd(_tmp17)
		}
		w.ListEnd(_tmp15)
		_tmp18 := w.List()
		for _, _tmp19 := range _tmp2.Code {
			_tmp20 := w.List()
			w.WriteUint64(uint64(_tmp19.TxIndex))
			w.WriteBytes(_tmp19.Code)
			w.ListEnd(_tmp20)
		}
		w.ListEnd(_tmp18)
		w.ListEnd(_tmp3)
	}
	w.ListEnd(_tmp1)
	w.ListEnd(_tmp0)
	return w.Flush()
}

func (obj *BlockAccessList) DecodeRLP(dec *rlp.Stream) error {
	var _tmp0 BlockAccessList
	{
		if _, err := dec.List(); err != nil {
			return err
		}
		// Accesses:
		var _tmp1 []AccountAccess
		if _, err := dec.List(); err != nil {
			return err
		}
		for dec.MoreDataInList() {
			var _tmp2 AccountAccess
			{
				if _, err := dec.List(); err != nil {
					return err
				}
				// Address:
				var _tmp3 [20]byte
				if err := dec.ReadBytes(_tmp3[:]); err != nil {
					return err
				}
				_tmp2.Address = _tmp3
				// StorageWrites:
				var _tmp4 []encodingSlotWrites
				if _, err := dec.List(); err != nil {
					return err
				}
				for dec.MoreDataInList() {
					var _tmp5 encodingSlotWrites
					{
						if _, err := dec.List(); err != nil {
							return err
						}
						// Slot:
						var _tmp6 [32]byte
						if err := dec.ReadBytes(_tmp6[:]); err != nil {
							return err
						}
						_tmp5.Slot = _tmp6
						// Accesses:
						var _tmp7 []encodingStorageWrite
						if _, err := dec.List(); err != nil {
							return err
						}
						for dec.MoreDataInList() {
							var _tmp8 encodingStorageWrite
							{
								if _, err := dec.List(); err != nil {
									return err
								}
								// TxIdx:
								_tmp9, err := dec.Uint16()
								if err != nil {
									return err
								}
								_tmp8.TxIdx = _tmp9
								// ValueAfter:
								var _tmp10 [32]byte
								if err := dec.ReadBytes(_tmp10[:]); err != nil {
									return err
								}
								_tmp8.ValueAfter = _tmp10
								if err := dec.ListEnd(); err != nil {
									return err
								}
							}
							_tmp7 = append(_tmp7, _tmp8)
						}
						if err := dec.ListEnd(); err != nil {
							return err
						}
						_tmp5.Accesses = _tmp7
						if err := dec.ListEnd(); err != nil {
							return err
						}
					}
					_tmp4 = append(_tmp4, _tmp5)
				}
				if err := dec.ListEnd(); err != nil {
					return err
				}
				_tmp2.StorageWrites = _tmp4
				// StorageReads:
				var _tmp11 [][32]byte
				if _, err := dec.List(); err != nil {
					return err
				}
				for dec.MoreDataInList() {
					var _tmp12 [32]byte
					if err := dec.ReadBytes(_tmp12[:]); err != nil {
						return err
					}
					_tmp11 = append(_tmp11, _tmp12)
				}
				if err := dec.ListEnd(); err != nil {
					return err
				}
				_tmp2.StorageReads = _tmp11
				// BalanceChanges:
				var _tmp13 []encodingBalanceChange
				if _, err := dec.List(); err != nil {
					return err
				}
				for dec.MoreDataInList() {
					var _tmp14 encodingBalanceChange
					{
						if _, err := dec.List(); err != nil {
							return err
						}
						// TxIdx:
						_tmp15, err := dec.Uint16()
						if err != nil {
							return err
						}
						_tmp14.TxIdx = _tmp15
						// Balance:
						var _tmp16 [16]byte
						if err := dec.ReadBytes(_tmp16[:]); err != nil {
							return err
						}
						_tmp14.Balance = _tmp16
						if err := dec.ListEnd(); err != nil {
							return err
						}
					}
					_tmp13 = append(_tmp13, _tmp14)
				}
				if err := dec.ListEnd(); err != nil {
					return err
				}
				_tmp2.BalanceChanges = _tmp13
				// NonceChanges:
				var _tmp17 []encodingAccountNonce
				if _, err := dec.List(); err != nil {
					return err
				}
				for dec.MoreDataInList() {
					var _tmp18 encodingAccountNonce
					{
						if _, err := dec.List(); err != nil {
							return err
						}
						// TxIdx:
						_tmp19, err := dec.Uint16()
						if err != nil {
							return err
						}
						_tmp18.TxIdx = _tmp19
						// Nonce:
						_tmp20, err := dec.Uint64()
						if err != nil {
							return err
						}
						_tmp18.Nonce = _tmp20
						if err := dec.ListEnd(); err != nil {
							return err
						}
					}
					_tmp17 = append(_tmp17, _tmp18)
				}
				if err := dec.ListEnd(); err != nil {
					return err
				}
				_tmp2.NonceChanges = _tmp17
				// Code:
				var _tmp21 []CodeChange
				if _, err := dec.List(); err != nil {
					return err
				}
				for dec.MoreDataInList() {
					var _tmp22 CodeChange
					{
						if _, err := dec.List(); err != nil {
							return err
						}
						// TxIndex:
						_tmp23, err := dec.Uint16()
						if err != nil {
							return err
						}
						_tmp22.TxIndex = _tmp23
						// Code:
						_tmp24, err := dec.Bytes()
						if err != nil {
							return err
						}
						_tmp22.Code = _tmp24
						if err := dec.ListEnd(); err != nil {
							return err
						}
					}
					_tmp21 = append(_tmp21, _tmp22)
				}
				if err := dec.ListEnd(); err != nil {
					return err
				}
				_tmp2.Code = _tmp21
				if err := dec.ListEnd(); err != nil {
					return err
				}
			}
			_tmp1 = append(_tmp1, _tmp2)
		}
		if err := dec.ListEnd(); err != nil {
			return err
		}
		_tmp0.Accesses = _tmp1
		if err := dec.ListEnd(); err != nil {
			return err
		}
	}
	*obj = _tmp0
	return nil
}
